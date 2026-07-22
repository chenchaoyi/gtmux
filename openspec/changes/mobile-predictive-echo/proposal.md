# mobile-predictive-echo

## Why

Typing into a remote pane from the phone shows nothing until the server round-trips —
`POST /api/send` then the next `capture-pane`. On a slow/cross-net link that is the
dominant felt latency: the 2026-07-21 diagnosis measured ~340 ms per keystroke echo,
which is the physical round-trip to an overseas VPS and cannot be erased by moving the
tunnel. Mosh solves exactly this with **predictive local echo** (see
`docs/design/mosh-predictive-echo-research.md`): show the user's own typing instantly on
the local screen, underlined until the server confirms, and let the authoritative screen
overwrite. It is transport-independent (works over our WS/TCP), so it applies where mosh's
UDP transport can't.

Mobile is the right first surface: unlike the raw `attach` client (byte passthrough, no
terminal model), the mobile terminal **already** has what predictive echo needs — the
server sends the cursor position (`PaneResponse.cursor {x, up}`), `term.ts` already draws a
cell at column x, and `/api/send` returns the post-send screen as a reconcile point. And
phone typing over a slow link is where the lag hurts most.

## What Changes

- The mobile terminal predicts local echo for **printable keystrokes + backspace** typed
  into a pane: draw the character at the known cursor column immediately, **underlined /
  dimmed** to mark it unconfirmed, advancing a local cursor.
- **Adaptive:** measure the send round-trip; predict only when it exceeds a small
  threshold (fast links show no predictions, no risk). Predictions are **local display
  only** — the real keystroke still goes via `POST /api/send`, unchanged.
- **Reconcile + honesty:** every prediction is dropped as soon as the next capture (or the
  `/api/send` response screen) confirms it; a wrong guess is overwritten by the
  authoritative screen within one round-trip. Any state-changing key (Enter, ESC, arrows,
  Ctrl-C, a 1/2/3 approval) **ends the prediction epoch** — clear outstanding predictions
  and stop predicting until the server confirms again. Never predict through a state
  change, and never in an alt-screen TUI where the cursor is hidden (`cursor.visible=false`).

## Impact

- Spec: `mobile-pane-renderer` (a predictive-local-echo requirement).
- Code (mobile only): `term.ts` / `NativeTerm.tsx` (overlay unconfirmed predicted cells at
  the cursor), the Composer/DetailScreen send path (feed keystrokes to the predictor +
  measure RTT + reconcile on the returned/polled screen), a small predictor module.
- No server or contract change — `/api/send` + `/api/pane` are unchanged; this is a
  client-side display optimization.
- Out of scope (deferred, noted in the research): `attach` predictive echo (needs a client
  VT model first), reconnect/resync roaming, any SSP adoption. Not adopting mosh itself
  (UDP + GPLv3 — design referenced, code reimplemented clean).
