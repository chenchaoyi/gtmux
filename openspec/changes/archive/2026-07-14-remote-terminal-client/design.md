## Context

`gtmux serve` already exposes sessions over HTTP/SSE, consumed by the web page and the
mobile app as owner or guest. `gtmux attach` adds a native terminal client that
byte-passthroughs a remote tmux pane so the LOCAL terminal becomes the remote session.
Design is grounded in a research pass (gotty, ttyd, tmux control-mode docs, creack/pty,
the Mosh papers; 24/25 claims confirmed under adversarial verification) — see the
report captured in `docs/design/remote-attach-research.md`.

## Goals / Non-Goals

**Goals:** a faithful, interactive, raw attach from any terminal, over the existing
serve/tunnel (WS-over-TCP, not SSH/UDP), honoring owner/guest scope, cgo-free.

**Non-Goals (MVP):** predictive local echo (Mosh); a control-mode (`-CC`) path;
roaming/loss-immunity (a fundamental TCP limit); scrollback beyond a reconnect resync.

## Decisions (research-backed)

- **Template = gotty relay + ttyd binary framing + creack/pty bridge.** gotty's
  architecture (output→client, input→PTY, one WS) is the Go blueprint, but gotty
  base64-encodes for a browser xterm — we send RAW bytes. ttyd's wire format is the one
  to copy: binary frames, first byte an opcode, payload at index 1.
- **Frame protocol (single WS, binary):** client→server `INPUT`(0x69) raw key bytes /
  `RESIZE`(0x72) `{"cols":C,"rows":R}` / `PAUSE`(0x70) / `RESUME`(0x52); server→client
  `OUTPUT`(0x6f) raw PTY bytes. Small fixed opcode set; raw payloads (no base64).
- **Bridge = `tmux` attached inside a server-side PTY, streamed raw.** Spawn `tmux
  new-session -t <session> \; select-window/pane` (a grouped, independently-sized client
  so it doesn't force-resize the owner's local tmux client) inside `pty.Start`, and copy
  the master both ways. NOT control mode (`-CC`): that is octal-escaped, line-framed
  TEXT (`%output`, `\134`) — must be parsed; wrong for raw passthrough (it's what iTerm2
  uses for native-tab mapping — the opposite goal). NOT `pipe-pane`+`send-keys` (what
  gtmux does today — not a real attach).
- **Flow control (the dominant pitfall).** Raw passthrough over TCP/WS is SSH-like:
  a flooding pane (`yes`, a noisy build) outruns a slow client → unbounded buffering /
  memory / lag, and Ctrl-C stuck behind buffered output. Mosh avoids this with
  screen-state diffs over UDP; we can't (Cloudflare tunnel = WS/TCP). So we HAND-BUILD
  ttyd-style flow control: server reads the PTY into a bounded channel; the client sends
  `PAUSE` above a high-water mark and `RESUME` below a low-water; on `PAUSE` the server
  stops reading the PTY (kernel/tmux backpressure then bounds memory). gorilla/coder
  websocket expose no `max_queue`/`write_limit` — we implement it.
- **Resize.** Client traps SIGWINCH, sends a `RESIZE` frame; the server has no local
  tty to `InheritSize` from, so it calls `pty.Setsize(ptmx, &Winsize{Rows,Cols})`
  (TIOCSWINSZ). tmux multi-client size negotiation is a wrinkle — the "clamp to smallest
  client" belief was REFUTED (tmux `window-size` is configurable); `new-session -t`
  gives the attach its own size, avoiding a forced resize of the owner's client.
- **Scope gating, server-authoritative + per-token.** Reuse the share scope: on connect
  verify the pane is view-allowed for a guest (else refuse the upgrade); DROP `INPUT`/
  `RESIZE` frames for a pane not on the guest input allowlist (view-only = read-only).
  Owner/device = full. Never trust a `--read-only` client flag for security (it's UX).
- **Latency / local echo.** Over a stable broadband tunnel the per-keystroke penalty is
  small (Mosh's 503ms→4.8ms is 3G-specific). Predictive echo is a later lever, not MVP.
- **Raw passthrough sidesteps ANSI/UTF-8 parsing.** We never parse the byte stream, so
  we can't cut an escape/multibyte sequence; the local real terminal reassembles across
  writes. (Verify once empirically with escape-heavy output split mid-sequence.)

## Risks / Trade-offs

- [Output flood OOM/lag] → bounded PTY-read buffer + PAUSE/RESUME; test with `yes`.
- [WS/TCP HOL-blocking + no roaming on lossy links] → fundamental; mitigate with
  reconnect + resync (tmux keeps the session server-side; on reattach the fresh `tmux`
  client redraws — cheap). Document that cellular/lossy paths degrade.
- [Raw terminal mode left dirty on crash] → restore termios on every exit path
  (`defer`, and on signal); a golden rule for raw-mode clients.
- [A guest attaching an owner-only pane] → refuse the WS upgrade server-side before
  spawning any PTY; input frames dropped for view-only.
- [Deadlocks between the two copy directions] → separate goroutines, ctx-cancel both on
  either end closing; bounded buffers so neither blocks forever.

## Open Questions

- Go WS lib: `gorilla/websocket` (chosen for MVP — ubiquitous, simple) vs `coder/
  websocket`; benchmark under escape-heavy flood later.
- tmux `window-size` policy for a shared session (largest/smallest/latest/manual) —
  test multi-viewer behavior; `new-session -t` is the current pick.
- Reconnect/resync mechanism: rely on the fresh tmux client's redraw, or also replay
  `capture-pane`? Fresh-client redraw is the MVP.
- Attach granularity: to the pane's SESSION (showing its window) vs a specific
  window/pane select. MVP: session of the pane + `select-window`/`select-pane`.
