# Change: web-shared input — menu-bar control surface

## Why

`web-shared-input` shipped the security core, the `gtmux share` CLI, and the web
mirror that lets a consented guest type into allowlisted panes (archived
`2026-07-13-web-shared-input`). PR4 — the **menu-bar app UI** — was deferred: v1
made the CLI the only host control surface. The `remote-access` spec left the
menu-bar surface as an explicit "planned follow-up."

The CLI is fine for power users but invisible to everyone who drives gtmux from
the menu bar. A collaborator asks to type into your terminal; today you must drop
to a terminal and remember `gtmux share on` / `add %N` / `new`. The host consent
that makes this feature safe (default OFF, per-pane) deserves a control surface as
discoverable as the Remote-access toggle it sits beside.

## What Changes

- **Menu-bar app** gains a **"Shared input" section in Preferences** (beside
  Remote access — guests arrive over the same serve/tunnel):
  - a **consent toggle** (default OFF; mirrors `gtmux share on/off`);
  - a **per-pane allowlist** rendered from the live agent list (each tmux pane a
    checkbox — `gtmux share add/remove %N`), so the host picks panes by the agent
    they already recognize, not a raw `%N`;
  - **guest share links**: list existing links with a one-tap **revoke**, and a
    **New share link** button that mints a link (`gtmux share new`) and copies the
    URL to the clipboard.
- A **compact popover exposure indicator**: when shared input is live (consent on
  AND ≥1 allowed pane AND ≥1 guest link), the popover footer shows a quiet glyph —
  a type-into-terminal exposure is never silent (same ethos as "Remote on").
- **CLI** gains additive `--json` output on `gtmux share status` and
  `gtmux share new`, so the app consumes the CLI contract (never the token file)
  for the guest list and the minted URL. No behavior change to the human output.
- The app stays a **pure CLI consumer**: it reads `share.json` directly for the
  cheap consent/allowlist poll (no secrets), and shells `gtmux share …` for every
  mutation and for the token-free guest list.

Out of scope (unchanged): the server gate is already authoritative; no new server
policy. No per-agent-row toggle in the popover (DESIGN restraint — the control is
self-contained in Preferences).

## Impact

- Affected specs: `menu-bar-app` (ADD: shared-input control surface),
  `remote-access` (MODIFY: the guest requirement — the menu-bar surface now exists;
  ADD the `share --json` contract).
- Affected code: `macapp/Sources/GtmuxBar/ShareStore.swift` (new),
  `Preferences.swift` (new section), `MenuView.swift` (footer indicator);
  `internal/app/sharecmd.go` (`--json`).
- No migration. Existing `gtmux share` CLI, `share.json`, and the guest roster are
  unchanged; the app is a new consumer of the same state.
