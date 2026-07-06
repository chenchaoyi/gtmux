## Why

When a Claude Code turn ends on an API/tool error (`Unable to connect to API`,
`response exceeded the … token maximum`, `Internal server error`, …) the agent
returns to its input prompt, so gtmux maps `Stop → idle` and shows the exact same
green ✓ "idle · your move" as a session that finished successfully. The failure is
invisible on the radar. Claude Code *does* record it — the transcript's last line
is an `assistant` entry with `"isApiErrorMessage": true` — gtmux just never reads
it (verified: 7 of 543 local sessions ended this way, with readable error text).

## What Changes

- Add a transcript check that reports whether a session's **last** message is an
  API-error entry (`isApiErrorMessage: true`), distinct from the noisy mid-turn
  retry errors that recover.
- Set a new **`error`** boolean on `idle` rows whose transcript ended in an error
  (and carry a short `error_text` summary), via `internal/app/agents.go`.
- Add `error` / `error_text` to the `gtmux agents --json` contract (backward
  compatible: absent/false = today's behavior).
- Surface it on **all three surfaces** as an **amber ⚠ "errored" modifier** on the
  idle row (with the error summary), replacing the green ✓ — CLI, menu-bar, mobile.
- The row stays in the **idle** state/section: an errored session genuinely is idle
  (at the prompt, your move). This marks HOW it ended, it is NOT a new status, and
  it MUST NOT use red (red is reserved for `waiting` / needs-you).

## Capabilities

### New Capabilities
<!-- none: this modifies existing capabilities -->

### Modified Capabilities
- `agent-radar`: the status-classification + `gtmux agents --json` contract gain an
  `error`/`error_text` modifier for idle sessions that ended on an API/tool error.
- `menu-bar-app`: the popover renders an errored idle row with an amber ⚠ + summary
  instead of the green ✓.
- `mobile-app`: the radar renders an errored idle row with an amber ⚠ + summary.

## Impact

- `internal/transcript/` (new "last message is an API error" check, Claude Code
  transcript schema — `isApiErrorMessage`).
- `internal/app/agents.go` (`agentJSON` gains `error`/`error_text`; set on idle rows).
- Contract consumers: `macapp/Sources/GtmuxBar/AgentStore.swift` + `MenuView.swift`;
  `mobileapp/src/api/types.ts` + the radar row. Backward compatible (new optional
  fields; older consumers ignore them).
- State-language invariant preserved: amber ⚠ is a modifier on idle (like `latest`),
  not a status color; colors still encode only state.
