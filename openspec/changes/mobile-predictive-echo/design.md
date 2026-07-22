# Design — mobile predictive local echo

## Context

Reference: `docs/design/mosh-predictive-echo-research.md`. We reimplement mosh's
predictive-echo *design* (GPLv3 code is not copied). The mobile terminal is a read-only
renderer over `capture-pane` snapshots (`NativeTerm` + `term.ts`); input is a separate
write via `POST /api/send`. The server provides the cursor (`PaneResponse.cursor {x, up,
visible}`), and `/api/send` returns the post-send screen.

## The predictor (client-only)

A small state machine, one per open pane:

- **State:** an ordered list of *outstanding predictions* (each: a character + the local
  (row,col) it was drawn at), a *local cursor* seeded from the last server `cursor`, and a
  measured *RTT estimate* (EWMA of recent send→screen round-trips).
- **On a printable keystroke** (and only when `predicting` — see gating): append the char
  at the local cursor, advance the local cursor one column, and mark it outstanding. On
  **backspace**: retreat the local cursor and mark the erased cell outstanding (drawn as a
  reversion). The real key still goes to `POST /api/send` unchanged.
- **Render:** `NativeTerm`/`term.ts` overlays outstanding predicted cells at their
  positions, **underlined or dimmed** (never the same weight as confirmed text), on top of
  the last authoritative screen.
- **Reconcile:** when a fresh screen arrives (the `/api/send` response, or the ~1.5 s
  capture poll), compare each outstanding prediction against the authoritative cell at that
  position: confirmed → drop it (it's now real); contradicted or timed out → drop it and
  trust the screen. Re-seed the local cursor from the server `cursor`.

## Gating (when to predict) — the honesty rules

- **Adaptive:** predict only when the RTT estimate exceeds a threshold (≈ tens of ms). On a
  fast link (LAN, good tunnel) `predicting=false` → no overlay, zero risk. This mirrors
  mosh (predictions "by default appearing solely during high-delay connections").
- **Epoch reset:** Enter, ESC, arrow keys, Ctrl-C, Tab, and the 1/2/3 approval taps all
  **end the epoch** — clear outstanding predictions and set `predicting=false` until the
  next confirmed screen. We never predict through a state change (the screen may jump
  arbitrarily), matching mosh's epochs.
- **Alt-screen / hidden cursor:** when `cursor.visible=false` (full-screen TUI), do not
  predict — we can't reason about the cursor. (Mosh CAN because it emulates the full
  terminal; we deliberately stay conservative rather than build a full VT model.)
- **Guest read-only panes:** no input → no predictor.

## Why this is safe (functionality unchanged)

- Predictions never leave the device — the byte sent to the pane is exactly what it was.
- The authoritative capture always wins within one round-trip; the worst case is a brief
  underlined character that gets corrected — strictly better than a blank wait.
- Conservative gating (printable+backspace only, epoch resets, no alt-screen, adaptive-off
  on fast links) keeps predictions in the one context where they're reliable: typing at a
  cooked prompt.

## Testing

- Pure predictor unit tests (jest): printable advances + underlines; backspace retreats;
  reconcile drops a confirmed prediction and keeps the screen on a contradiction; an
  epoch-ending key clears outstanding predictions; `predicting=false` below the RTT
  threshold and in alt-screen. No device needed.
- An e2e smoke (optional): with a slowed demo client, a typed char shows underlined then
  resolves.

## Alternatives considered

- **Full VT model (mosh-grade), shared with `attach`:** more capable (predicts in TUIs)
  but far larger and needed on the raw `attach` client too — deferred. Mobile's
  server-provided cursor lets us get 90% of the value at a fraction of the cost.
- **No adaptivity (always predict):** rejected — on a fast link the underline flicker is
  pure downside; mosh itself gates on delay.
