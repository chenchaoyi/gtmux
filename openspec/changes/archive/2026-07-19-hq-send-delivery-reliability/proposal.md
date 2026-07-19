# Change: hq-send-delivery-reliability

## Why

gtmux's DISPATCH channel is now reliable — #474 gave it ack + idempotency + retry,
and `send-submit-reliability` gave delivery full-payload (head+tail) paste
confirmation so a long task can no longer submit truncated. But two channels that
sit on EITHER side of dispatch are still a generation behind, and each produces a
reproducible instability the HQ can only catch after the fact by
capture → diagnose → correct:

- **A — a new coding-agent session's FIRST long command truncates (the spawn
  drive path).** `gtmux spawn` launches `claude`, waits for the agent to come up,
  then pastes the goal. But its readiness check — `dispatchbridge.WaitAgentReady`
  — only asks whether the pane's foreground process is no longer a shell
  (`pane_current_command` left the `ShellCommands` set). It returns `true` the
  INSTANT `claude` is the foreground process — which is exactly mid-boot, while the
  TUI is still painting the startup banner (`N MCP servers need authentication`),
  the trust-folder gate, and the MCP-connect spinner. `Deliver` then pastes a long
  goal into a composer that has not yet stably taken over input; the TUI drops tail
  characters across its mount/repaint, and the Enter is swallowed by a transient
  banner. First-time + long is the worst case: first-time = boot is least stable;
  long = the payload spans the unstable window (a short goal lands inside one paint
  tick and slips through). Observed 2026-07-19 on `%88`: the goal truncated
  mid-string, Enter eaten by the MCP banner; HQ had to clear the box and re-send a
  shorter goal by hand.

- **B — a `waiting` (permission/question) cleared in the source window fires NO
  `resolved` wake, so HQ keeps a stale needs-you.** When a `waiting·permission` /
  `waiting·question` state is set (by a `Notification`/`Waiting` hook) and the user
  approves IN THE SOURCE WINDOW and the agent just resumes, the `resolved` wake is
  gated in the hook on a NARROW event list — it fires only on
  `UserPromptSubmit` / `Resumed` / `Stop` / `StopFailure`. For Claude the
  permission-continue produces none of those promptly (Claude's gtmux hook set
  registers no `PostToolUse`; the wait marker persists until the turn's eventual
  `Stop`), so HQ is never told the wait cleared and holds a stale `needs-you` for
  minutes. Observed 2026-07-19 on `%88`: after approval the event stream showed no
  further `%88` event at all; the board's `waiting-perm` sat stale until the user
  caught it. This is the known "threw an ask, then sat waiting for a `resolved`
  that never came" pitfall, but its root is that gtmux does not emit an event for
  the "permission approved, agent continued" clear.

Both are the same shape: the drive/wake channels reach a not-yet-ready TUI, or fail
to observe a state change, and the HQ backstops them. **The goal is to weld the
reliability into the tools so the first attempt is correct — not to rely on HQ
re-checking.**

## What Changes

### A. Spawn delivery becomes a four-state handshake: launched → ready → content-verified → submitted

- **Ready is screen-based, not process-based.** A new readiness probe replaces the
  "process is no longer a shell" test as the gate before delivery. A pane is READY
  only when its capture shows an input-ready composer: the prompt line is present ∧
  no `starting`/`connecting`/`loading`/authentication banner is on screen ∧ the
  startup/trust gate is not up ∧ the MCP-connect noise has settled ∧ two consecutive
  samples are unchanged (stable). A max wait + backoff bounds it; on timeout spawn
  reports `failed` with the capture as evidence rather than pasting blind.
- **Content-verified and submitted reuse the shipped machinery.** The paste is
  atomic (bracketed `paste-buffer`), the draft is read back and confirmed to hold
  the FULL goal (head AND tail) before Enter, and a swallowed Enter is re-confirmed
  (never a blind re-Enter of the whole payload). This is already `dispatch.Deliver`
  / `confirmPaste` / `pasteWithGuard` from `send-submit-reliability` — this change
  does NOT rebuild it; it ensures spawn's delivery flows through it AFTER the pane
  is ready, so the confirmation is no longer racing a booting TUI.
- **Pre-clear the startup gate.** The trust-folder gate and the MCP banner are the
  main interferers, so they are folded into the readiness criteria (a gate/banner on
  screen ⇒ not ready), and spawn keeps its existing discipline of ensuring the cwd
  is trusted so the gate does not appear in the first place.

### B. Any `waiting → non-waiting` transition emits an acked `resolved` wake

- **Emit from the single-writer state machine, not only from specific hook events.**
  The serve slow-tick (the single writer that already samples every pane) tracks the
  last-known waiting state per pane and, when a pane that WAS waiting is no longer
  waiting, fires ONE `resolved` wake — regardless of which hook (or no hook) cleared
  it. This catches the "permission approved, agent continued" clear that the hook's
  narrow event list misses. The hook's existing `nudgeResolved` fast path stays as
  the immediate emit for the events it already covers; the slow-tick is the backstop
  that closes the gap and de-dups against the fast path so a clear is announced once.
- **`resolved` rides the acked/retried/deduped channel.** The `resolved` wake goes
  through `hqnudge` (the `hq-wake-reliability` channel: ack + retry + claim-reclaim +
  degradation) like every other wake, so it is not lost the way an old best-effort
  wake could be.

## Capabilities

### Modified Capabilities

- `agent-dispatch` — `gtmux spawn` delivery is specified as a readiness-gated
  handshake: the target pane must present an input-ready, stable composer (no
  startup/trust gate, no boot banner, no unsettled MCP noise, two stable samples)
  BEFORE the goal is pasted; process-liveness alone does not authorize delivery.
- `hq-wake-protocol` — a `resolved` wake SHALL fire on ANY observed
  `waiting → non-waiting` transition (emitted by the single writer as a backstop to
  the hook's event-specific emit), deduped so a clear is announced once, and
  delivered on the acked/retried wake channel.

## Non-goals

- No rebuild of the paste/Enter machinery — `send-submit-reliability`'s head+tail
  confirmation and swallowed-Enter re-confirm are reused as-is; A only gates them on
  readiness.
- No new CLI flags or HTTP surface; the wire contracts are unchanged. (Readiness
  timeout reuses the existing `spawnReadyTimeout` tune knob.)
- No HQ-side change. The complementary HQ discipline — polling its own thrown
  needs-you rather than depending on a nudge — is playbook work (hq-capture-loop),
  not part of this change.
- Not Claude-specific: the readiness signatures and the `waiting`-transition emit
  are agent-generalized (the startup-gate/banner table is keyed like the existing
  per-agent gate table), so they work for any hook-equipped agent TUI.

## Impact

- `internal/prompt/prompt.go` — a scoped `IsComposerReady(capture, agent)` (prompt
  line present ∧ no boot banner/gate), reusing `IsStartupGate`; a per-agent boot-banner
  table alongside `startupGates`.
- `internal/dispatchbridge/dispatchbridge.go` — `WaitAgentReady` gains the
  screen-readiness + two-stable-samples gate on top of the process check.
- `internal/hq/slowtick.go` — a `waiting → non-waiting` transition sweep (single
  writer) that emits the `resolved` backstop wake and de-dups against the hook.
- `internal/hook/nudge.go` / `internal/hook/hook.go` — no behavior removed; a shared
  dedup marker so the fast-path `nudgeResolved` and the slow-tick backstop announce a
  clear once.
- Tests: `prompt` (readiness/banner detection), `dispatchbridge` (ready gate),
  `slowtick` (transition emit + dedup), `hook` (resolved-once).
- Specs: `agent-dispatch`, `hq-wake-protocol`.
- No new persistence format; reuses the capture, the `waiting/<pane>` marker, the
  slow-tick single writer, and the `hqnudge` acked channel.
