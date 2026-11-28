## Context

Antigravity CLI (`agy`) exposes `SessionStart`, `PreInvocation`, `PostInvocation`, `PreToolUse`, `PostToolUse`, and `Stop` hooks, but emits no `PermissionRequest` or `Notification` hook when presenting interactive TUI choices (`Run this command?`, `Allow access to this URL?`, `Allow calling this tool?`, or multiple-choice questions). Its TUI renders the active choice with an ASCII `>` cursor (`> 1. Yes, run command`), soft-wraps long command-rule options in narrow panes, and draws a `↑/↓ Navigate ...` footer beneath the options. Pressing `1`–`9` directly selects and submits that option.

## Goals / Non-Goals

**Goals:**
- Detect active `agy` selection/approval menus accurately without false-positiving on Markdown blockquotes (`> 1. ...`) in prose.
- Reconstruct soft-wrapped option labels on narrow mobile panes and allow replyable command-prefix labels up to 240 runes.
- Promote `agy` panes to `waiting` immediately in `radar`, `digest`, `HQ`, and `serve` (`hasPendingAsk`), and return them to `working` or `idle` immediately once the menu leaves the screen.

**Non-Goals:**
- Changing mobile app TypeScript bundles (`DetailScreen` and `SessionInteractionCard` already consume `status === "waiting"` and `/api/interaction` options).

## Decisions

- **Navigation-chrome guard for ASCII `>` cursor**: Unicode TUI cursors (`❯›▶▸→`) remain standalone menu indicators, while ASCII `>` leading a numbered option counts as a live menu selector in `WaitingOptions` only when navigation chrome (`↑/↓` + `navigate`, `enter to select`, or `enter to confirm`) appears below the option run and no composer prompt row follows the last option.
- **Bottom-anchored recency for wrapped menus**: Expand `WaitingOptions`'s scan slice to 28 bottom lines so wrapped 4-option menus retain option `1.`, while still requiring the menu's *last* option row to sit within 14 non-blank lines of the bottom so buried scrollback menus remain ignored.
- **Immediate screen-ask gate before 20s slow-tick**: When `waiting/<pane>` does not yet exist, `hasPendingAsk` checks `radar.PaneHasScreenAsk(id)` so `GET /api/interaction` and `GET /api/options` serve choices at sub-second latency; existing `startup`, `draft`, and empty markers continue to fail closed.
