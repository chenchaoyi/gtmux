## Why

Antigravity CLI (`agy`) pauses for interactive command, URL, tool, and question selections without emitting a `PermissionRequest` hook, rendering `> 1. ...` numbered choices with a `↑/↓ Navigate` footer that often soft-wrap on narrow mobile panes and exceed 100 characters when quoting command prefixes. Because `WaitingOptions` required Unicode cursors (`❯›▶▸→`), scanned only 14 bottom lines, did not join soft-wrapped option continuations, capped replyable labels at 100 runes, and radar/server screen-wait promotion only covered `codex`, `agy` sessions stayed `working` with no selection card in the App.

## What Changes

- Teach `internal/prompt` to recognize `>`-led numbered menus when accompanied by menu navigation chrome (`↑/↓ Navigate` / `Enter to select`), join narrow-pane soft-wrapped option continuations, raise the replyable single-column rune limit to 240, and recognize `agy`'s project trust startup gate.
- Extend screen-wait `waiting` promotion, digest `kind`/`ask` fallback, HQ `slowtick` persistence/resolved transitions, and `serve`'s pre-marker `hasPendingAsk` gate to `agy` (and `kiro`).

## Capabilities

### Modified Capabilities
- `agent-integration`: Recognize screen-rendered interactive approval and choice menus for hook-equipped agents (`agy`) that emit no permission-wait hook.

## Impact

## Surfaces

- **Terminal (`cmd/gtmux`)**: `gtmux agents`, `gtmux digest`, `gtmux options`, and `gtmux send` accurately report `waiting` and active choices for `agy` panes.
- **macOS menu-bar app (`macapp/`)**: Consumes `GET /api/agents` and `GET /api/options`, surfacing `waiting` status and choices for `agy`.
- **Phone (`mobileapp/`)**: `DetailScreen` (`Chat` and `Terminal` tabs) receives `status: "waiting"` and `GET /api/interaction` `options`, immediately rendering `SessionInteractionCard` + `ApprovalCard` tap-to-reply buttons.
- **iPad (`mobileapp/`)**: Same shared `DetailScreen` and `SessionInteractionCard` behavior as Phone.
- **Web (`internal/server/web/`)**: Receives `status: "waiting"` and `/api/interaction` + `/api/options` choices in Chat and Pane views.
