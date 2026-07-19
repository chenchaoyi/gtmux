# Design: hq-send-delivery-reliability

## Context

Three channels drive/observe agent panes, each with its own reliability posture:

| Channel | What it does | Posture today |
|---|---|---|
| **dispatch** (`Deliver`) | paste a task, confirm the FULL draft, confirm submit | reliable — head+tail confirm (`send-submit-reliability`) + ack/idempotency (#474) |
| **spawn drive** (`spawn` → `WaitAgentReady` → `Deliver`) | launch an agent, then dispatch into it | delivery is reliable, but the READINESS gate before it is process-only → pastes into a booting TUI |
| **wake** (`hqnudge`) | knock the HQ pane on decision-dense events | delivery is acked/retried (`hq-wake-reliability`), but the `resolved` PRODUCER misses the "approved in source, agent resumed" clear |

This change closes the two remaining gaps: **A** hardens the spawn-drive readiness
gate; **B** fixes the `resolved` producer. Both reuse existing machinery — no new
persistence, no new wire surface.

Key existing pieces this design builds on:

- `dispatch.Deliver` / `confirmPaste` / `pasteWithGuard` / `draftHasDelivery`
  (`internal/dispatch/deliver.go`): bracketed atomic paste, head+tail draft
  confirmation, swallowed-Enter re-confirm, at-most-once paste. **Reused as-is.**
- `prompt.IsStartupGate(capture, agent)` + the per-agent `startupGates` table
  (`internal/prompt/prompt.go`): detects the trust-folder / permission gate.
  **Extended, not replaced.**
- `dispatchbridge.WaitAgentReady(pane, timeout)`
  (`internal/dispatchbridge/dispatchbridge.go`): the process-liveness gate spawn
  calls before delivery. **This is what A strengthens.**
- The serve slow-tick single writer (`internal/hq/slowtick.go`, `stuckDispatchSweep`
  pattern): observes every pane, writes markers race-free, emits wakes.
  **This is where B's transition emit lives.**
- `internal/hook/nudge.go nudgeResolved` + the hook's clear-emit gate
  (`internal/hook/hook.go:584`): the current, event-specific `resolved` emit.
  **Kept as the fast path; the slow-tick backstops it.**
- `hqnudge` acked/retried channel + `hqwake.ClassResolved` (already
  `PriorityOutcome`). **Reused for delivery.**

---

## A. Spawn delivery as a four-state handshake

Today spawn is `launch → (process up) → paste`. The fix makes it
`launched → ready → content-verified → submitted`, where only the READY state is
new; content-verified and submitted are the states `Deliver` already enforces.

### State machine

```
launched         WaitAgentReady: pane_current_command left the shell set
   │             (claude/codex/… is the foreground process). NECESSARY, NOT sufficient.
   ▼
ready            NEW. The composer is input-ready and stable:
   │               • prompt line present (the agent's input glyph row)
   │               • NO startup/trust gate on screen        (prompt.IsStartupGate)
   │               • NO boot banner: starting / connecting / loading /
   │                 "MCP servers need authentication" / auth spinner
   │               • two consecutive captures are byte-identical (settled)
   │             Bounded by ReadyTimeout (default 20s) with backoff between polls.
   │             On timeout → spawn reports state:"failed" + capture evidence; NO paste.
   ▼
content-verified  Deliver → pasteWithGuard (bracketed paste-buffer) → confirmPaste:
   │               the draft read back holds the FULL goal (ContainsHead ∧ ContainsTail,
   │               or looksCollapsedPaste). A head-only draft is a fragment → Esc +
   │               re-paste, up to the existing retry cap. REUSED, unchanged.
   ▼
submitted        Deliver sends Enter, confirms the turn started (draft cleared / hook
                 UserPromptSubmit). A swallowed Enter re-confirms the FULL draft still
                 holds before re-sending Enter alone — never re-pastes the payload.
                 REUSED, unchanged.
```

The whole point: the paste/Enter machinery is already correct, but it was firing
against a booting TUI. Moving the READY gate from "process up" to "composer stable
and input-ready" means `confirmPaste` no longer has to out-wait a mount/repaint —
by the time it runs, the composer is the stable owner of input.

### A1 — the readiness probe (the new state)

Add `prompt.IsComposerReady(capture, agent string) bool` in
`internal/prompt/prompt.go`:

```
IsComposerReady(cap, agent) :=
      hasPromptLine(cap, agent)          // the input glyph row is present
   && !IsStartupGate(cap, agent)         // trust/permission gate not up  (reuse)
   && !hasBootBanner(cap, agent)         // no starting/connecting/auth banner
```

- `hasPromptLine` looks at the bottom region of the capture (same
  bottom-14-lines window `WaitingOptions` already scopes to) for the agent's input
  prompt row. For Claude that is the `❯`/`>` composer row with no numbered menu on
  it (distinguish from a live `WaitingOptions` menu — a menu is NOT "ready to take a
  goal").
- `hasBootBanner` matches a per-agent `bootBanners` table, keyed exactly like
  `startupGates` (`""` = Claude default): the phrases that mean "still connecting" —
  `MCP servers need authentication`, `Connecting…`, `Starting…`, `Loading…`, the
  auth/spinner lines. Extensible per agent; NOT hardcoded to Claude.

`WaitAgentReady` (in `dispatchbridge`) then becomes: keep the current
`pane_current_command` liveness check to reach `launched`, then poll `CaptureFull`
until `IsComposerReady` is true for **two consecutive identical captures**
(stability), or the deadline passes. The two-stable-sample rule is the cheap
defense against catching the composer between two repaints of the banner.

Data flow: `spawn.go` already calls `WaitAgentReady(pane, ReadyTimeout)`; no call-site
change — the gate's semantics strengthen underneath it. `dispatchbridge` owns the
tmux capture (it already owns `DispatchIO`), so `prompt.IsComposerReady` stays a pure
string predicate testable without tmux.

**Retry / bound:** the readiness poll is bounded by the existing `ReadyTimeout` tune
(default 20s, `spawnReadyTimeout` override) with `backoff(attempt)` (already in
`dispatch`) between polls. Timeout is a hard failure: spawn returns
`state:"failed"`, `delivered:false`, evidence = the last capture — it MUST NOT paste
into a pane that never became ready (that is exactly the truncation we are killing).

### A2 — atomic paste + content-verify retry (reuse)

No new code beyond wiring. `Deliver`'s `pasteWithGuard` uses bracketed
`paste-buffer` (atomic; a multi-line goal cannot submit line-by-line and cannot
leave an unterminated bracket that eats Enter), and `confirmPaste` requires the FULL
head+tail before authorizing Enter. The **fingerprint compare** is the shipped one:
`NormalizeHead`/`NormalizeTail` (space-normalized first/last `headRunes`) with
`ContainsHead ∧ ContainsTail`, tolerant of the composer's own soft re-wrap, or
`looksCollapsedPaste` for a composer that collapses a big paste to a placeholder. A
settled head-without-tail is a fragment → `clearedForRetry` (Esc) → re-paste, capped
by the existing retry loop. This change's contribution is that this now runs against
a READY composer, so the "fragment" verdict reflects a real drop, not a mid-boot
repaint.

### A3 — Enter confirmation (reuse)

`Deliver`'s verify loop confirms submission and handles a swallowed Enter by
re-confirming the FULL draft still holds before re-sending Enter alone (never a blind
re-Enter, never a re-paste). Unchanged; documented here as the fourth state so the
handshake is complete end to end.

### A4 — startup-gate pre-clear

Two layers, both already partly present:

1. **Prevent the gate:** spawn keeps ensuring the launch cwd is trusted (HQ's
   existing discipline) so Claude's trust-folder gate never appears. No new code —
   this is a spec assertion that a spawned session SHOULD NOT hit an avoidable gate.
2. **Never deliver through a gate/banner:** folded into A1 — `IsStartupGate` and
   `hasBootBanner` both make `IsComposerReady` false, so the readiness gate itself is
   the enforcement. If a gate is genuinely up at the deadline (e.g. an
   un-pre-trustable path), spawn fails with evidence rather than pasting a goal that
   the gate keypress would swallow.

---

## B. `waiting → non-waiting` transition emits an acked `resolved`

### Root cause (precise)

`resolved` is emitted from the HOOK (`internal/hook/hook.go`):

```go
if d.clearWaiting && hadWaiting &&
    (event == "UserPromptSubmit" || event == "Resumed" ||
     event == "Stop"            || event == "StopFailure") {
    nudgeResolved(pane, priorWaitKind)
}
```

For Claude, a permission approved in the source window → agent continues produces
none of those events promptly: Claude's gtmux hook set (`agent_hooks.go`) registers
`UserPromptSubmit / PermissionRequest / Stop / SessionStart / SessionEnd` — no
`PostToolUse`, so the tool completing after approval fires NO gtmux hook, and the
`waiting·permission` marker persists until the turn's eventual `Stop` (minutes).
During that window HQ holds a stale `needs-you`. The emit is keyed on "a specific
hook event fired", but the real signal is "the pane is no longer waiting", which the
hook cannot always observe.

### Fix — a transition detector in the single writer

Move the AUTHORITY for `resolved` from "a hook event fired" to "the pane's waiting
state changed", observed by the serve slow-tick (the single writer that already
samples every pane and owns race-free markers — same home as `stuckDispatchSweep`).

**State tracked (new marker, mirrors the existing pattern):** per pane, the
last-seen waiting kind the slow-tick observed, e.g.
`~/.local/share/gtmux/hqwake/resolved-last-<pane>` holding the kind that was pending.
(Reuses `state.WriteMarker`/`ReadMarker`; no new format.)

**Sweep logic each slow-tick:**

```
for each pane the tick samples:
    wasWaiting := ReadMarker(resolvedLast<pane>)      // kind we last saw pending, or ""
    nowWaiting := state.Exists(WaitingPath(pane))     // the hook-driven marker
                  ? state.ReadMarker(WaitingPath(pane)) : ""

    if wasWaiting != "" && nowWaiting == "":
        // waiting → non-waiting: the wait cleared with no resolving hook event
        if !recentlyResolvedByHook(pane):            // dedup vs the fast path
            nudgeResolved(pane, wasWaiting)          // acked hqnudge channel
        WriteMarker(resolvedLast<pane>, "")
    else:
        WriteMarker(resolvedLast<pane>, nowWaiting)   // track current state
```

Notes:

- **Single writer.** Only the slow-tick writes `resolved-last-<pane>`, so no
  read-modify-write race (identical discipline to `stuckDispatchSweep` /
  `markerChanged`).
- **The `waiting/<pane>` marker is still hook-driven.** The hook clears it on
  approval-continue paths it CAN see (`Resumed`, `Stop`, …) and on `PostToolUse` for
  agents that register it. The slow-tick reads that marker's presence — so a clear by
  ANY path (hook event, or the marker simply going away) triggers the backstop emit.
  This is a pure ADD: the slow-tick never sets `waiting`, only observes its clearing.
- **Screen fallback (optional, defense in depth).** If a pane still carries the
  `waiting` marker but its screen has visibly advanced past the gate (no
  `WaitingOptions` menu, no `IsStartupGate`, an active turn spinner), the tick MAY
  treat it as cleared. Kept conservative — the marker's disappearance is the primary
  signal; the screen check only rescues a marker the hook failed to clear at all.

### Dedup — announce a clear ONCE

The hook fast path and the slow-tick backstop must not both announce the same clear.
A shared dedup marker (`resolved-emit-<pane>` stamped with the cleared kind + a short
TTL, or reuse the existing `markerChanged("resolved-<pane>", kind)` helper):

- Hook `nudgeResolved` stamps the marker when it emits.
- The slow-tick sweep checks the marker (`recentlyResolvedByHook`) and skips its emit
  if the hook already announced this clear within the TTL.
- If neither fired yet (the permission-continue case), the slow-tick is the sole
  emitter.

Net: exactly one `resolved` per waiting→clear, whichever channel observes it first.

### B2 — `resolved` on the acked/retried/deduped channel

`nudgeResolved` already calls `nudgeHQ` → `deliverWake` → `hqnudge.Deliver` /
`hqnudge.Enqueue`, i.e. the `hq-wake-reliability` channel: draft-guarded, acked,
retried, claim-reclaimed, and escalated to `wake-degraded` on repeated failure. So
B2 is satisfied by construction the moment the slow-tick emits through
`nudgeResolved` — the only requirement is that the slow-tick's emit path is the SAME
`nudgeResolved`/`hqnudge` path, not a bespoke best-effort `SendText`. The spec pins
this so a future refactor can't regress it to a raw send.

`hqwake.ClassResolved` is already `PriorityOutcome` in the priority table — a
`resolved` correctly ranks below decision-class wakes and drains in order. No
priority change.

---

## Relationship to sibling changes

- `send-submit-reliability` (archived) — owns the head+tail paste/Enter
  confirmation. This change REUSES it and only adds the readiness GATE in front of
  it for the spawn path. No overlap in code touched (that change was `deliver.go`;
  A1/A4 are `prompt.go` + `dispatchbridge.go`).
- `hq-wake-reliability` (archived) — owns the acked/retried/deduped/escalated wake
  DELIVERY. This change REUSES it for `resolved` (B2) and only fixes the `resolved`
  PRODUCER (B1). No overlap (that change hardened `hqnudge`/delivery; B1 is the
  slow-tick emit).
- `stuck-dispatch-waiting` (archived) — added `IsStartupGate` + the per-agent gate
  table + the slow-tick `stuckDispatchSweep`. This change EXTENDS the gate table
  with boot banners (A1) and adds a sibling transition sweep (B1) in the same single
  writer. Complementary: that change catches "stuck BEFORE running → waiting"; this
  one catches "no longer waiting → resolved".

Given the clean reuse boundaries, this stays a standalone change (not a merge into
either archived sibling) — each task is a pure increment behind its own PR.

## Open questions (for review, not blockers)

1. **Readiness prompt-line signature for non-Claude agents.** A1's `hasPromptLine`
   needs a per-agent input-row signature; the initial table can ship Claude-only
   (default `""`) and treat an unknown agent as "ready once no banner + stable",
   accepting a slightly weaker gate for agents without a signature. Confirm that
   degradation is acceptable, or require a signature per registered agent.
2. **Slow-tick cadence vs. staleness.** The `resolved` backstop lands within one
   slow-tick of the clear (the serve slow-tick interval). If a tighter bound is
   wanted for `resolved` specifically, it could ride the 3s fast tick (as the wake
   queue flush does) instead — at the cost of a per-pane capture every 3s. Default:
   slow-tick; escalate only if the delay is felt.
