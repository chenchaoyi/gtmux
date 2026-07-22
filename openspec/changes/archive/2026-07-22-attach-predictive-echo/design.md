# Design — attach predictive local echo

Reference: `docs/design/mosh-predictive-echo-research.md`. We reimplement mosh's DESIGN
(GPLv3 code not copied), adapted to gtmux's raw-passthrough-over-WS attach and its one
advantage: the server has tmux, which knows the cursor.

## The core problem, and why gtmux is easier than mosh here

Predictive echo must (a) know where the cursor is, to draw a prediction, and (b) reconcile
— erase a wrong or now-confirmed prediction — when authoritative output arrives. Mosh
solves both by running a **full terminal emulator** on the client (it maintains a virtual
screen and repaints diffs). The gtmux attach client is raw passthrough — the LOCAL
terminal (Ghostty/iTerm) is the emulator; the client just pipes bytes. Building a full
emulator into the client is the big, deferred option.

**Shortcut:** the attach server bridges a *tmux* pane, and tmux exposes the cursor
(`#{cursor_x}`, `#{cursor_y}`) and whether the pane is in the alternate screen
(`#{alternate_on}`). The server samples these and sends an `OpCursor {x, y, alt}` frame.
The client then has the authoritative cursor for free — no client emulator for the common
(cooked-line) case.

## Wire protocol: `OpCursor`

- Add `OpCursor byte = 'c'` (server→client) to `internal/connect/frame.go`, payload JSON
  `{"x":C,"y":R,"alt":bool}` (encode/decode helpers, unit-tested — same style as Resize).
- `internal/server/attach.go` samples tmux for the bridged pane on a small cadence (e.g.
  ~every 100 ms while attached, and it may coalesce/skip when unchanged) and after it
  writes an `OpOutput` batch, and sends an `OpCursor`. Sampling is a cheap
  `display-message -p`; it never blocks the PTY pump (own goroutine / non-blocking).
- Old clients ignore an unknown opcode (the reader already `continue`s on non-Output);
  new clients that never receive `OpCursor` simply never predict.

## Client predictor (in `internal/connect/attach.go`)

State per attach session:
- `serverCursor {x,y,alt}` — last authoritative cursor from `OpCursor`.
- `predicted string` — the chars typed since `serverCursor` that the server hasn't echoed.
- `epoch bool` — false after a state-changing key, until the next `OpCursor`.
- `rtt` — EWMA of the delay between a sent keystroke and the confirming `OpCursor`/output.

Flow (all on the existing stdin→INPUT and OUTPUT→stdout goroutines, guarded by one mutex):
1. **Keystroke (stdin):** always forward the raw byte to the pane (unchanged). THEN, if
   `predicting()`: a printable byte → append to `predicted`, write the byte to stdout in
   the **unconfirmed style** (`ESC[4m` underline + a dim SGR, then `ESC[0m`), advancing the
   local cursor; a backspace → pop `predicted` and emit `BS ESC[K` (rub out) locally; a
   state-changing key (Enter/ESC/arrows/Ctrl-C/Tab) → `endEpoch()` (see below).
2. **`OpCursor` (from server):** reconcile — the server has processed some of our input.
   Compute the confirmed prefix from how far `serverCursor.x` advanced on the same row;
   drop that prefix. If predictions remain but the cursor/row moved unexpectedly, clear
   them. Re-seed `serverCursor`, resume the epoch.
3. **`OpOutput` (from server):** before writing the authoritative bytes to stdout, **erase
   any outstanding predicted tail** (move the terminal cursor back `len(predicted)` cells
   and `ESC[K` clear to end-of-line), then write the real bytes. The server's bytes carry
   the true echo + cursor, so the screen is correct afterward; `predicted` is cleared (the
   next `OpCursor` re-seeds).

`endEpoch()` = erase outstanding predictions locally, clear `predicted`, set `epoch=false`.
`predicting()` = `epoch && !serverCursor.alt && rtt >= THRESHOLD`.

## Honesty + safety (mosh's rules, plus attach realities)

- **Printable + backspace only**; never predict arrows/ESC/Enter/Ctrl-C (state changes).
- **Adaptive:** `rtt < THRESHOLD` (LAN / fast tunnel) → never predict (no underline, no
  erase, zero risk). Matches mosh "high-delay only".
- **Cooked-line only:** `alt=true` (a full-screen TUI) → never predict; we don't emulate.
- **Bail cheap:** any uncertainty (multi-line wrap near the right margin, an `OpOutput`
  that doesn't look like a simple echo) → `endEpoch()` and pass through untouched. A wrong
  guess is erased within one round-trip; worst case is a brief underlined flicker, and a
  terminal redraw (`Ctrl-L` to the pane, or reattach) always recovers.
- **Off by default:** gated by `--predict` (and/or a config key). Inline overlay-and-erase
  on a raw terminal is the risk; ship it opt-in until proven, then consider default-on for
  high-RTT links only.

## Staging

1. **Plumbing:** `OpCursor` frame + server sampling + client cursor tracking, with NO
   prediction (optionally a debug readout). Verifies the cursor stream is correct + cheap.
2. **Prediction:** single-line printable/backspace prediction + reconcile/erase behind
   `--predict`. The core.
3. **Polish:** RTT adaptivity, the full epoch heuristic set, tuning the sample cadence and
   the unconfirmed style.

## Testing

- `frame.go`: `OpCursor` encode/decode round-trip (unit).
- The predictor's PURE core factored out (advance/backspace/reconcile-prefix/epoch-reset/
  gating) and unit-tested with no terminal, mirroring the mobile predictor that was
  prototyped then reverted — the logic is portable, only the render target differs.
- Manual: a high-latency link (or an artificial delay) with `--predict`, typing at a shell
  prompt (predicts + confirms), in `vim` (alt-screen → no prediction), and a mispredict
  (server output differs → erased, not left).

## Alternatives considered

- **Full client VT emulator (mosh-grade):** predicts everywhere incl. TUIs, but it's a
  large rewrite of the passthrough client. Deferred. The server-cursor shortcut gets the
  common case at a fraction of the cost.
- **No `OpCursor`, parse output client-side for the cursor:** re-derives what tmux already
  knows and needs partial VT parsing anyway. Rejected — the server has the answer.
