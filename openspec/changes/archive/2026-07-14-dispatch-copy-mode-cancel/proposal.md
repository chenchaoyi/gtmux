# Exit tmux copy-mode before delivering input; expose the input-lock on the radar

## Why

When a target pane is in tmux **copy-mode / view-mode** (the user scrolled up the
scrollback), tmux interprets every incoming key as a *mode-navigation* command, not
program input. So keys never reach the agent's input box — they are **silently
swallowed**. This bites three ways:

1. **Manual typing** into a scrolled pane does nothing (a known tmux footgun).
2. **`gtmux send` / `gtmux spawn`** deliver via `paste-buffer` + `Enter`, both of
   which a mode eats — the payload vanishes.
3. Worse, the delivery can **mis-verify as "landed"**: the fragment/screen fallback
   can misread a copy-mode frame, so a dispatch that never reached the agent is
   reported delivered.

`internal/hqnudge` already senses this (`#{pane_in_mode}`) but only to *defer* typing
(the supervisor politely waits for the human to stop scrolling). Programmatic dispatch
has the opposite need: the task MUST land. The fix is to **drop the pane out of the
mode first** (`send-keys -X cancel`), then deliver as normal, keeping the existing
land-verification intact.

Separately, nothing today tells a supervisor (or any surface) *which* pane is
input-locked. We add an additive `in_mode` flag to the `agents --json` and
`digest --json` contracts so the lock is visible.

## What Changes

- **New tmux helpers** (`internal/tmux`): `InMode(pane)` (reads `#{pane_in_mode}`,
  converging hqnudge's inlined check) and `ExitCopyMode(pane)` (`send-keys -X cancel`,
  a no-op when the pane is not in a mode — `-X cancel` errors "not in a mode"
  otherwise, so it gates on `InMode`).
- **Dispatch delivery exits copy-mode before pasting.** `dispatch.Deliver` gains
  optional `InMode`/`ExitMode` IO hooks; the paste guard cancels the mode before each
  paste attempt. Verification is unchanged — the payload now actually lands, so the
  existing checks confirm it truthfully. Covers `gtmux send` (verified path) and
  `gtmux spawn`.
- **The plain/unverified write paths exit copy-mode too**: `gtmux send`'s
  `--no-verify`/`--no-enter`/`--key` path and `POST /api/send` (`sendToPane`) call
  `ExitCopyMode` before typing, so the phone and scripted sends land as well.
- **`in_mode` on the contracts.** `agents --json` (`agentJSON`) and `digest --json` /
  `GET /api/digest` (`digestRow`) gain an additive, optional `in_mode` bool — true
  only for a tmux pane currently in copy/view-mode. Absent (omitempty) otherwise, so
  existing consumers are unaffected.

Out of scope: any UI treatment of `in_mode` in the menu-bar / mobile surfaces (they
now *can* read it; a visual indicator is a later design pass). hqnudge keeps its
existing "defer, don't cancel" policy — the human scrolling the HQ pane is not a
dispatch target.

## Impact

- Affected specs: `agent-dispatch` (delivery exits copy-mode), `agent-radar` +
  `agent-digest` (`in_mode` field).
- Affected code: `internal/tmux/tmux.go`, `internal/dispatch/deliver.go`,
  `internal/app/{dispatchbridge,send,serve,agents,digest}.go`,
  `internal/hqnudge/hqnudge.go` (converge on `tmux.InMode`).
- Affected tests: `internal/dispatch/deliver_test.go`,
  `internal/app/{agentjson,digest}_test.go`.
- Contract change is additive-only (`in_mode` omitempty); no consumer breaks.
