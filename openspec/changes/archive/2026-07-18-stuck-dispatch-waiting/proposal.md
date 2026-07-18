# Proposal: stuck-dispatch-waiting

## Why

A dispatched worker that is actually STUCK before running a single step — (a) at a
session STARTUP/permission gate (Claude's "Do you trust the files in this folder?"
confirmation, or another agent's equivalent), or (b) with its task text UNSUBMITTED
(a long paste swallowed the Enter, so the goal sits in the composer) — is misclassified
as `done`.

Root cause: `waiting` is HOOK-marker-driven ONLY (`~/.local/share/gtmux/waiting/<pane>`,
written by the Stop/Notification hook). The startup gate and the unsubmitted composer
fire NO gtmux hook, so the radar reads the pane as `idle`, and `taskStatusFor("idle")`
maps `idle → done` UNCONDITIONALLY. So `gtmux tasks` + the digest show the stuck dispatch
as **done**, and NO `waiting` (needs-you) wake is emitted — HQ and the user believe the
work finished when not one step ran. (Real incident: a disk-inventory worker stuck at
83% at the trust gate was marked `done`; the user only caught it by asking.)

## What Changes

Recognize the "stuck-before-running" state from the pane's SCREEN (a narrow, explicit
exception to the "waiting is never inferred from screen" invariant) and classify it as
`waiting` (needs-you) instead of `done`:

1. **Radar guard** — in `gatherAgents`, after the hook-driven `resolveWaiting`, when the
   resolved status would be `idle`/`running` AND the pane's capture shows (a) a startup/
   permission gate OR (b) a STRUCTURED, non-empty input draft on a TRACKED-dispatch pane,
   reclassify to `waiting` with a distinct kind (`startup` / `draft`). This fixes every
   downstream surface at once — `gtmux tasks`, `digest`, the menu-bar — because they all
   read `p.status`; `taskStatusFor` no longer sees `idle` for a stuck pane.
2. **Wake + marker** — the serve slow-tick (the single writer) writes the `waiting`
   marker for a detected-stuck TRACKED pane and emits a `waiting` wake, so HQ is nudged
   and the watchdog escalates it. The read path (`gatherAgents`) only CLASSIFIES; it
   never writes (it runs on every `gtmux agents` call).
3. **Done-wake guard** (defense in depth) — the `Stop → done` wake is skipped when the
   post-Stop capture shows a startup gate or a draft still holding the payload, so an
   incidental `Stop` can't relabel a stuck pane `done`.
4. **Cross-agent** — the startup-gate signatures become a per-agent table (keyed like
   `agentProfile`), not hardcoded to Claude, so a `codex`/`gemini`/… worker at its own
   startup gate is caught too.

`done` is thereby reserved for a session that truly completed a turn.

## Capabilities

### Modified Capabilities
- `agent-radar`: a NARROW screen-based exception to "waiting is never inferred from
  screen" — a startup/permission gate, or a tracked dispatch holding an unsubmitted
  draft, reads as `waiting`, not `idle`.
- `agent-dispatch`: a dispatched pane blocked at a startup gate / holding an undelivered
  draft is `waiting`, NEVER `done`.
- `hq-wake-protocol`: such an idle-but-stuck pane must NOT fire a `done` wake and SHOULD
  fire a `waiting` wake.

## Impact

- `internal/prompt/prompt.go` (a scoped `IsStartupGate` + a per-agent gate table),
  `internal/app/agents.go` (the radar guard + capture reuse), `internal/app/slowtick.go`
  (the single-writer wake + marker), `internal/hook/nudge.go` (the done-wake guard).
- Tests: prompt, agents (`classifyAgent`/`resolveWaiting`), taskscmd, dispatch.
- Specs: agent-radar, agent-dispatch, hq-wake-protocol.
- No new persistence/format; reuses the existing capture + draft/gate detectors.
