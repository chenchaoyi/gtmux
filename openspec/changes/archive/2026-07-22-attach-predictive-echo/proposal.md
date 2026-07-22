# attach-predictive-echo

## Why

`gtmux attach` is a raw per-keystroke terminal: every character waits a full round-trip
before it echoes. The 2026-07-21 diagnosis measured ~340 ms per keystroke over the
overseas-VPS tunnel — the felt "typing is laggy" that mosh's predictive local echo exists
to hide (see `docs/design/mosh-predictive-echo-research.md`). The network/infra levers
(LAN-prefer, a closer VPS, fixing the proxy) reduce the RTT but can't erase distance;
**predictive local echo is the only client-side lever that makes typing feel instant at
340 ms.** It is transport-independent, so it works over our WS/TCP tunnel where mosh's UDP
cannot.

The reason this was deferred (`remote-attach-research.md`): predictive echo needs a
client-side terminal-state model, and the raw passthrough client has none. **gtmux has a
shortcut mosh doesn't: the server bridges a tmux pane, and tmux already knows the cursor.**
So the server can send the cursor to the client — no client-side terminal emulator needed
for the common case.

## What Changes

- **New `OpCursor` frame (server→client)** on the attach WS: the server samples tmux
  (`#{cursor_x}`, `#{cursor_y}`, `#{alternate_on}`) on a small cadence + on change and
  sends `{x, y, alt}`. This is the authoritative cursor + a "cooked line vs full-screen
  TUI" signal, and the reconcile truth for predictions.
- **Client predictive echo** (behind a flag/config, OFF by default): keep a LOCAL
  predicted cursor seeded from `OpCursor` and advanced by the user's own typing; on a
  printable key or backspace, draw the predicted character at the cursor in a distinct
  **underlined/dim** style on the local terminal, before the server echoes it; the real
  keystroke still goes to the pane unchanged. When authoritative output (`OpOutput`) or a
  fresh `OpCursor` arrives, **erase the predictions first**, then apply the real bytes —
  the server screen is always authoritative.
- **Honesty gating (mosh's heuristics):** predict printable + backspace ONLY; adaptive —
  predict only when the measured round-trip is high (a fast/LAN link shows none); only in a
  cooked-line context (`alt=false`, cursor visible); and any state-changing key (Enter,
  ESC, arrows, Ctrl-C, Tab) **ends the epoch** (clear predictions, pause until the next
  `OpCursor`). Bail to plain passthrough on anything not confidently a single-line edit.

## Impact

- Spec: `remote-terminal-client` (the `OpCursor` frame + predictive echo requirement).
- Code: `internal/connect/frame.go` (`OpCursor` encode/decode), `internal/server/attach.go`
  (sample tmux cursor + send frames), `internal/connect/attach.go` (predictor: local
  cursor, prediction overlay, reconcile-and-erase, gating; RTT estimate). All cgo-free.
- Back-compat: `OpCursor` is additive — an old client ignores an unknown opcode; a new
  client without `OpCursor` frames simply never predicts. Predictive echo is **off by
  default** (a `--predict` flag / config) because inline overlay-and-erase on a raw
  terminal is genuinely risky; the authoritative server screen and a redraw always recover.
- Staged (see design.md): (1) `OpCursor` plumbing + client cursor tracking (no prediction);
  (2) single-line printable/backspace prediction behind the flag; (3) adaptivity + epoch
  heuristics + polish. Deferred: a full client VT emulator / multi-line + TUI prediction
  (mosh-grade), and `attach` reconnect/resync roaming (a separate small change).
