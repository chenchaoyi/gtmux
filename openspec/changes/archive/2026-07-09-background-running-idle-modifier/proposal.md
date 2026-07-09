## Why

When a coding agent finishes its turn but the user still has a background shell
running (a `run_in_background` command — a dev server, a long build/test), gtmux
reports the session as plain `idle` (green ✓), identical to a session that is
truly done. That's misleading: the session is "paused waiting for background work
to wake it", not finished. Claude Code now surfaces exactly this distinction in
its `Stop` hook payload — a structured `background_tasks` array whose documented
purpose is to *"let hooks distinguish 'session is done' from 'session is paused
waiting for background work to wake it'"*. We should read that official signal and
show it, so an idle row with live background work reads as such rather than as
"all clear".

## What Changes

- gtmux's Claude hook (`gtmux hook`, which already parses Claude's stdin JSON)
  additionally reads the `Stop` payload's `background_tasks` array. When it holds
  any running/pending item at turn end, the hook records a background-work marker
  for the pane (count + a short label, e.g. the shell command); otherwise it
  clears the marker (empty array = truly done).
- The radar (`gtmux agents --json`) surfaces this as a **modifier on the `idle`
  state**, mirroring the existing `error`/`error_text` pattern: an idle row gains
  `bg: true`, `bg_count: N`, and `bg_text` (a short summary). Status stays `idle`.
- All three surfaces render the modifier as `idle · ⧗N` ("background running") in
  an amber/neutral tone — **never red** (red is reserved for `waiting`/needs-you).
  Consistent with the `errored` ⚠ precedent.
- **Scope (v1): Claude Code only.** Codex and other agents expose no official
  "background still running" signal at turn end, so their rows never carry `bg`
  (left for a later process-tree fallback or official support). A `bg`-unaware
  consumer sees a normal idle row — backward compatible.
- Fix an existing gap: the Web mirror (`internal/server/web/app.js`) does not
  render the `error`/`error_text` modifier today; wire both `error` and the new
  `bg` modifier there so the Web surface matches CLI/menu-bar/mobile.

## Non-goals

- Not a new status. `bg` annotates `idle`; the four-state language
  (waiting/working/idle/running) and its section order are unchanged.
- No screen-scraping and no OS process-tree probing in v1 — the signal is the
  agent's own official hook payload only.
- Codex/other-agent background detection is out of scope here (deferred).
- `session_crons` (CronCreate/ScheduleWakeup/`/loop` wakeups, also in the Stop
  payload) is noted but not surfaced in v1 — the marker is about in-flight
  background *work*, not scheduled wakeups.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities

- `agent-radar`: add a `bg` "background running" modifier on the `idle` state —
  new `Stop`-payload signal (`background_tasks`), new contract fields
  (`bg`/`bg_count`/`bg_text`), and the rule that it renders amber/neutral, never
  red, with status unchanged.

## Impact

- **Code**
  - `internal/hook/` — Claude `Stop` handling reads `background_tasks`; writes/
    clears a per-pane background-work marker.
  - `internal/state/` — new marker path under `~/.local/share/gtmux/` (e.g.
    `bg/<pane>`), mirroring `finished/<pane>`.
  - `internal/app/agents.go` — `agentPane.bg*` fields → `agentJSON` (`bg`,
    `bg_count`, `bg_text`); CLI render overlay (amber ⧗) mirroring `errored`.
  - `macapp/Sources/GtmuxBar/AgentStore.swift` — decode `bg*`; render the
    modifier per `DESIGN.md`.
  - `mobileapp/src/ui/AgentRow.tsx` (+ `api/types.ts`, theme) — render the
    modifier per `MOBILE.md`.
  - `internal/server/web/app.js` — render `error` (existing gap) and `bg`.
- **Contract**: `gtmux agents --json` gains optional `bg`/`bg_count`/`bg_text`.
  Additive and backward compatible (absent/false on every non-matching row).
- **State paths**: one new marker directory under `~/.local/share/gtmux/`.
- **Docs**: update `agent-radar` spec; note the field in the JSON-contract
  requirement.
