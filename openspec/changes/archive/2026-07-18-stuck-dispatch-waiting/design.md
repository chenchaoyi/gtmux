# Design: stuck-dispatch-waiting

## Context

- The radar (`internal/app/agents.go` `gatherAgents`) is the single source of truth;
  `tasks`, `digest`, menu-bar, and the watchdog all derive from `p.status`.
- `waiting` today is HOOK-ONLY: `resolveWaiting` (agents.go) returns `waiting` iff the
  pane is in `state.WaitingSet()` (a marker written by the Stop/Notification hook). Its
  doc-comment (and `agent-radar/spec.md`) states "waiting is never inferred from screen
  output — it belongs to the hook, not the terminal."
- A stuck-before-running pane fires NO hook, so it reads `idle` (or `running`→`idle` via
  `hookFreeStatus`), and `taskStatusFor("idle") → "done"` (taskscmd.go), unconditional.
- Existing seams to reuse (no new capture, no new parsing):
  - `internal/prompt/prompt.go`: `startupChoosers` already lists the Claude trust gate
    ("Do you trust the files"); `looksLikeStartupChooser(text)`.
  - `internal/dispatch/region.go`: `DraftOf(capture) (draft, structured)` — the pane's
    input draft (structured=false when no input box).
  - `internal/dispatch/ledger.go`: `TaskForPane(pane) (Task, bool)` — is this a tracked
    dispatch?
  - `hookFreeStatus` already `tmux.CapturePane(paneID)`s the idle/running candidate.

## Goals / Non-Goals

**Goals:** a startup/permission gate OR an unsubmitted draft on a tracked dispatch reads
as `waiting` + fires a wake + never `done`; reuse existing capture/detectors;
cross-agent.

**Non-Goals:** inferring waiting from screen for the NORMAL permission/plan/question
cases (those stay hook-driven — this is a NARROW exception for pre-turn gates + undelivered
drafts). No change to how `done` is detected for a genuinely completed turn.

## Decisions

1. **Narrow the "never from screen" invariant, don't drop it.** Only two screen states
   flip an otherwise-idle pane to `waiting`: (a) a startup/permission GATE, (b) a
   STRUCTURED non-empty draft on a TRACKED-dispatch pane. Everything else stays exactly
   hook-driven. Scoping (b) to `TaskForPane` != none avoids flagging a human mid-compose
   in a normal pane.
2. **Classify in the read path, WRITE only in the slow-tick.** `gatherAgents` (runs on
   every `gtmux agents`) reclassifies for DISPLAY (pure, reuses `hookFreeStatus`'s
   capture — capture once, pass the string into the guard). The `waiting` MARKER write
   (which drives the wake + watchdog escalation) happens ONLY from the serve slow-tick
   (`slowtick.go`, the single writer) — never as a side effect of a read.
3. **A scoped `prompt.IsStartupGate`.** Extract from `looksLikeStartupChooser`, but scope
   it to the trust/PERMISSION gate (the "stuck, needs a keypress to proceed" kind), NOT
   the resume/theme pickers — `WaitingOptions` deliberately excludes the resume picker
   (a 2h-old reopened session must not read as waiting forever); keep that exclusion.
4. **Per-agent gate table.** `startupChoosers` becomes keyed by agent (default = Claude's
   phrases), overridable via the agent profile, so a `codex`/`gemini` gate is matched by
   its own phrases rather than Claude's.
5. **Done-wake guard.** `wakeDone` (or its `Stop` call site) skips the `done` wake when
   the post-Stop capture is a startup gate or a draft still holding the payload —
   defense in depth against an incidental `Stop`.
6. **A distinct kind.** The waiting kind is `startup` (gate) / `draft` (unsubmitted), so
   the digest/HQ can say WHY, and the wake line reads `waiting·startup` / `waiting·draft`.

## Risks / Trade-offs

- **False positive on a legitimately-idle draft**: a user typing in a NORMAL pane isn't
  flagged (draft-detection is scoped to tracked dispatches). A dispatched pane where the
  agent legitimately left a draft is vanishingly rare and still correctly "needs you".
- **Resume-picker regression**: explicitly out of scope for the gate detector (kept
  excluded), pinned by the existing `prompt_test` cases.
- **Screen churn**: the guard runs on an already-taken capture; no extra `capture-pane`.

## Migration

None — reuses existing state paths + detectors; additive classification only.
