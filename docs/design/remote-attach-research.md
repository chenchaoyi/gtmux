# `gtmux attach` — PTY-over-WebSocket research (2026-07-14)

## Reading this historical note (2026-10-06)

The text below records the July 14 investigation and its proposed MVP. Its source
counts, timing figures and manual-test claims are historical records, not a fresh
validation of the current release. Later code differs in several relevant ways:

- `internal/app/serve.go` attaches to the requested pane's **session**. Authorizing the
  initial pane does not isolate the resulting tmux client to that pane. The guest
  isolation defect and proposed restriction are tracked in
  [#1372](https://github.com/chenchaoyi/gtmux/pull/1372); that proposal is pending a
  decision. The historical words "scope-gate" and "leak-free" below are not evidence
  that other panes cannot be exposed.
- The bridge uses synchronous output writes for backpressure. `PAUSE` / `RESUME`
  have reserved opcodes but are no-ops in `internal/server/attach.go`; the archived
  task list's checked flow-control items do not prove that client-driven pausing was
  delivered. The corresponding current spec requirement remains an implementation
  gap; this note does not remove it.
- Predictive local echo was subsequently added as opt-in `--predict`, with cursor,
  alternate-screen and latency gates (`internal/connect/predict.go` and the
  [July 22 change](../../openspec/changes/archive/2026-07-22-attach-predictive-echo/proposal.md)).
  "Deferred" below describes the earlier MVP.
- "ttyd-style" describes the framing pattern, not protocol compatibility. gtmux uses
  `i/r/p/R/o` (and later `c`); [ttyd's own opcodes](https://github.com/tsl0922/ttyd/blob/main/src/server.h)
  and resize payload differ (`columns` there, `cols` here).

The [current contract](../../api/contract.md) and
[capability spec](../../openspec/specs/remote-terminal-client/spec.md) describe the
intended current interface; the unresolved implementation gaps above still apply.
The original investigation follows unchanged.

Research pass (deep-research harness: 28 sources → 116 claims → 25 adversarially
verified, 24 confirmed / 1 refuted) informing the `remote-terminal-client` change:
stream a remote tmux pane bidirectionally to a raw local terminal over the existing
`gtmux serve` HTTP surface (WebSocket through the Cloudflare tunnel — WS/TCP, not
SSH/UDP), honoring the owner/guest token scope.

## Borrowable template — gotty + ttyd + creack/pty

- **Architecture: gotty** (Go) — a WebSocket relay (output→client, input→PTY, one WS).
  Caveat: gotty base64-encodes for a browser xterm; we send **raw bytes**. Borrow the
  relay shape, not the encoding.
- **Wire protocol: ttyd** — binary WS frames, first byte an opcode, payload at index 1
  (client→server INPUT/RESIZE/PAUSE/RESUME; server→client OUTPUT). This is what gtmux
  copies (`internal/connect/frame.go`): raw PTY bytes in OUTPUT, no base64.
- **PTY primitive: creack/pty** — `pty.Start(cmd)` → master `*os.File`; read/write it is
  the byte pump. Remote resize: `pty.Setsize(ptmx, &Winsize{Rows,Cols})` (TIOCSWINSZ) —
  the server has no local tty to `InheritSize` from.
- **Default-deny input (gotty `--permit-write`)** → extended to **per-token scope**: the
  server drops INPUT/RESIZE frames for a view-only guest pane. Never trust the client.

## tmux bridging — attach inside a server-side PTY (NOT control mode)

Spawn `tmux attach-session -t <session-of-pane>` inside a `creack/pty` PTY and stream
the master byte-for-byte. **Not** control mode (`-CC`): it's octal-escaped, line-framed
TEXT (`%output`, `%begin/%end`, chars <32 → `\ooo`) — must be parsed/unescaped, wrong
for raw passthrough (it's what iTerm2 uses for native-tab mapping — the opposite goal).
**Not** `pipe-pane`+`send-keys` (what gtmux does today — not a real attach).

## Pitfalls (design against these up front)

The core insight: **raw passthrough over TCP/WS is SSH-like** and inherits SSH's
flood/backpressure and delayed-Ctrl-C. Mosh avoids it with screen-state diffs over UDP
(skips intermediate states; Ctrl-C halts in ≤1 RTT) — we can't (Cloudflare tunnel is
WS/TCP), so mitigate explicitly:

1. **Output flood / backpressure** — a flooding pane (`yes`, noisy build) outruns a slow
   client → unbounded buffering/OOM/lag. MVP mitigation: synchronous `WriteMessage` gives
   a natural backpressure chain (slow client → WS write blocks → server stops reading the
   PTY → tmux blocks), and input runs in its OWN goroutine so Ctrl-C is never stuck behind
   output. ttyd-style explicit PAUSE/RESUME is defined in the protocol for a future async
   (browser) client.
2. **Keystroke latency / no local echo** — over a stable broadband tunnel the penalty is
   small (Mosh's 503ms→4.8ms is 3G-specific). Predictive local echo needs a client
   terminal-state model a raw client lacks — a later lever, not MVP.
3. **Resize** — client traps SIGWINCH → RESIZE frame → server `pty.Setsize`. tmux
   multi-client size negotiation is a wrinkle (`window-size` is configurable; the
   "clamp to smallest client" belief was REFUTED). MVP uses `attach-session` (leak-free);
   `new-session -t` (independent size) is the follow-up if it disrupts the owner's client.
4. **WS-over-TCP head-of-line blocking + no IP roaming** on lossy links — a fundamental
   TCP limit (can't fix without UDP). Rare over a stable tunnel; on cellular it degrades.
   Mitigate with reconnect + resync (tmux keeps the session; a fresh client redraws).
5. **Server-side per-token input gating** — refuse the WS upgrade for a non-viewable
   guest pane (no PTY spawned); drop write frames for a view-only pane.

UTF-8/ANSI splitting at chunk boundaries is a **non-issue for raw passthrough** (we never
parse; the local terminal reassembles across writes) — worth one empirical check.

## Recommendation (implemented in this change)

gotty-relay + ttyd-binary-framing + creack/pty; bridge via `tmux attach-session` in a
server-side PTY (raw passthrough); scope-gate + input-drop server-side; natural
backpressure + independent input goroutine. Deferred: predictive local echo, control-mode
(`-CC`) path, reconnect/resync, `new-session -t` sizing.

## Open questions

Go WS lib (gorilla — chosen — vs coder/websocket, benchmark under flood); tmux
`window-size` policy for a shared session; reconnect/resync mechanism; attach granularity
(session vs specific window/pane). Coverage gaps: sshx / VibeTunnel / tmate / wetty /
xterm attach addon didn't yield surviving claims (VibeTunnel is a direct competitor;
sshx = host PTY + encrypted WS to a stateless relay + xterm.js — worth a later look).

Key sources: gotty (github.com/yudai/gotty), ttyd (github.com/tsl0922/ttyd), creack/pty,
tmux Control-Mode wiki, the Mosh papers (USENIX ATC '12).
