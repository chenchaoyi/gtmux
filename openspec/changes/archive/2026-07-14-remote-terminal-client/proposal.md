## Why

You can reach a Mac's gtmux sessions from the web page or the mobile app, but not by
truly **attaching** from another machine's terminal to work in the session. `gtmux
attach <target>` opens the remote tmux pane in your LOCAL terminal (Ghostty / iTerm2 /
Terminal) as a faithful, interactive, raw byte-for-byte passthrough — over the SAME
`gtmux serve` HTTP surface (a WebSocket through the Cloudflare tunnel), honoring the
same owner/guest token scope. It's the third client surface and the "any surface ×
either scope" north star, done as a real terminal attach (not a curated TUI).

## What Changes

- New command **`gtmux attach <target>`**: target is a guest share link
  (`https://host/#t=<token>` → GUEST) or a host + `--token` (→ OWNER). It WebSocket-
  connects to the serve, puts the local terminal in raw mode, and byte-passthroughs a
  remote tmux pane both ways. `--read-only` forces watch-only.
- New server endpoint **`GET /api/attach?id=%N`** (WebSocket, authed + scope-gated):
  spawns `tmux attach`/`new-session -t <session>` for the pane INSIDE a server-side PTY
  (`creack/pty`) and bridges that PTY's master byte-for-byte to the WS.
- **Wire protocol** (research-backed, ttyd-style): binary WS frames, first byte an
  opcode — client→server `INPUT` / `RESIZE` / `PAUSE` / `RESUME`; server→client
  `OUTPUT` (raw PTY bytes) — payload from byte 1. Resize carries `{cols,rows}`.
- **Scope gating (server-authoritative, per-token):** a guest may attach ONLY to a
  view-allowed pane; `INPUT`/`RESIZE` frames are DROPPED server-side for a pane not on
  the guest's input allowlist (a view-only pane is read-only). Never trust the client —
  extends gotty's default-deny model from a global flag to per-token scope.
- **Flow control (the #1 pitfall):** a raw passthrough over TCP/WS inherits SSH-style
  output-flood + backpressure. Server bounds its PTY-read buffer; the client sends
  `PAUSE`/`RESUME` on high/low water marks and the server pauses reading the PTY. Hand-
  built in Go (no `max_queue` knob in gorilla/coder websocket).
- Adds pure-Go deps: `creack/pty`, a WebSocket lib (`gorilla/websocket`),
  `golang.org/x/term` (raw mode). The CLI MUST stay cgo-free.

### Non-goals (MVP)

- Predictive local echo (Mosh-style) — only worth it if latency hurts; needs a client
  terminal-state model a raw passthrough lacks. Deferred.
- Optional tmux control-mode (`-CC`) path for iTerm2 native-tab mapping — the opposite
  design (framed/escaped text); a later, separate mode.
- Multi-remote switching, scrollback replay beyond a reconnect resync, roaming (a
  fundamental TCP limit — can't be fixed without UDP, which the Cloudflare tunnel isn't).

## Capabilities

### New Capabilities
- `remote-terminal-client`: `gtmux attach` — connect-by-target + scope, the WS PTY
  bridge (frames, flow control, resize, input gating), and the raw local-terminal client.

### Modified Capabilities
- `remote-access`: add the `GET /api/attach` WebSocket endpoint to the serve contract
  (authed + scope-gated), and note the CLI is now a first-class terminal client.

## Impact

- **Server (Go):** a new `/api/attach` WS handler in `internal/server` + a PTY bridge
  (`creack/pty` spawning `tmux attach`), the binary frame codec, flow control, and
  per-token input gating tied to the existing share scope. New WS dep.
- **CLI (Go):** `gtmux attach` + `internal/connect` (target/scope parse, WS client, raw
  terminal mode via `x/term`, SIGWINCH→RESIZE, PAUSE/RESUME). i18n + help.
- **Contracts:** additive — one new WS endpoint; the frame protocol is documented in
  `api/contract.md`. No change to existing endpoints.
- **cgo-free:** all new deps are pure Go; `CGO_ENABLED=0 go build ./cmd/gtmux` must pass.
