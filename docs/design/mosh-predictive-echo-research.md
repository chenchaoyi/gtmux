# Mosh evaluation for gtmux — predictive local echo (2026-07-22)

Follows `remote-attach-research.md`. Evaluates https://mosh.org against gtmux's remote
surfaces (the `attach` WS bridge + the mobile terminal) to decide what, if anything, is
directly usable or worth reimplementing. Prompted by the "Get rid of network lag" claim:

> "SSH waits for the server's reply before showing you your own typing… Mosh gives an
> instant response to typing, deleting, and line editing… On a bad connection,
> outstanding predictions are underlined so you won't be misled."

## What Mosh actually does (4 pillars)

1. **Predictive local echo.** The client hypothesizes each keystroke echoes at the cursor
   and backspace/←/→ do the obvious thing; it shows predictions **only under high delay**,
   **underlines unconfirmed** ones, and resets into a new "epoch" (pausing prediction) on
   state-changing keys (ESC/CR/arrows). Predictions are drawn **locally only** — the
   server output is authoritative and overwrites — so it's safe even in vim/emacs.
2. **Roaming / sleep.** UDP datagrams carry increasing sequence numbers + a 3s heartbeat;
   the server retargets its reply IP on any higher-seq packet → survives IP change / NAT /
   sleep. Warns when it "hasn't heard from the server in a while".
3. **SSP (State Synchronization Protocol).** Syncs a **screen-state snapshot diff**, not a
   byte stream — skips intermediate frames (a `yes` flood can't drown you), regulates the
   frame rate to avoid buffer bloat, keeps Ctrl-C instant. Two SSPs (keys up, screen down).
4. **Transport = UDP** + AES-128-OCB3.

## The two hard constraints for us

- **Transport mismatch.** Mosh is UDP; gtmux's whole remote story is **WS/TCP through a
  tunnel** precisely to traverse NAT/corp-firewalls that block UDP (mosh needs UDP
  60000–61000 open — exactly what the office network drops). **We cannot wrap or adopt
  mosh; its transport is what our tunnel can't carry.**
- **License.** Mosh is **GPLv3** — copying its code would force gtmux to GPL. We may
  **reference the design and reimplement clean**; the algorithm is well-documented.

## Per-pillar verdict

| Mosh pillar | gtmux verdict |
|---|---|
| **Predictive local echo** | ⭐ **Reimplement (reference design).** The one high-value borrow — the only gtmux-side fix that makes cross-net typing *feel* instant. Applies to both surfaces; **mobile-first** (below). |
| **Roaming** | Reference the *idea* only: a WS heartbeat + fast reconnect-and-redraw (tmux keeps the session) + a "haven't heard from the server" warning. Not the UDP mechanism. A follow-up for `attach` resilience. |
| **SSP screen-diff** | **Mobile already has it for free** — the mobile terminal polls `capture-pane` (the whole current screen) every ~1.5s, i.e. it already syncs *screen state*, not bytes, so it inherits mosh's flood-resistance. For `attach` (byte passthrough) a full SSP = a rewrite (the deferred "档 2"), not worth it; our synchronous-write backpressure + independent input goroutine already bound memory and keep Ctrl-C responsive. |
| **UTF-8 insistence** | Already learned — we force `LC_CTYPE` + `tmux -u` (see `attach-cjk-term-locale`). |

## Why the fix is real (recap of our latency finding)

From the 2026-07-21 diagnosis: attach's per-keystroke lag was **~340 ms**, and that is the
**physical round-trip to an overseas VPS over a cross-border VPN** — not gtmux code. The
network/infra fixes (LAN-prefer, a closer VPS, fixing the proxy fake-ip that added a bogus
5 s TLS) reduce the RTT but can't erase distance. **Predictive local echo is the only
gtmux-side lever that makes typing feel instant *at* 340 ms** — it hides the RTT for the
common case (typing at a prompt) while staying honest (underline) and authoritative
(server wins). That is exactly mosh's value proposition, and it is transport-independent,
so it works over our WS/TCP tunnel.

## Key insight: do it mobile-first

Predictive echo needs a **client-side terminal-state model** (where is the cursor, what's
on the line). The `attach` client is raw byte passthrough — it has **no** such model and
would need an embedded VT parser first. **The mobile terminal already has everything:**

- `PaneResponse.cursor = {x, up, visible}` — the server **already tells us the cursor
  position** each capture (`src/api/types.ts`).
- `term.ts` already maintains the rendered screen and `cursorSpans(line, x, …)` already
  draws a cell at column x (`src/ui/term.ts`).
- `/api/send` **returns the post-send screen** (`DetailScreen.tsx`) — a clean reconcile
  point, on top of the ~1.5 s capture poll.

So on mobile we can predict by drawing the typed character at `cursor.x` (underlined/dim),
advancing a local cursor, and dropping the prediction the moment the next capture confirms
it. Far cheaper than building a VT model into the raw `attach` client — and mobile typing
(a phone keyboard over a slow link) is where the lag hurts most. **Ship predictive echo on
mobile first; bring it to `attach` later once (or if) a client VT model exists.**

## The borrowable heuristics (straight from mosh, reimplemented)

- Predict **only printable chars + backspace** (advance / retreat the local cursor); treat
  ←/→ as cursor moves if we want, but **ESC / Enter / anything else = end the epoch**
  (stop predicting until the server confirms) — never guess through a state change.
- **Adaptive:** measure the send round-trip; when it's low (LAN / fast tunnel) **don't
  predict at all** (no underline, no risk). Only kick in above a threshold (~tens of ms).
- **Underline / dim unconfirmed predictions** — visual honesty; a wrong guess is corrected
  within one RTT by the authoritative capture, so the worst case is a brief flicker.
- Predictions are **local display only** — the real keystroke still goes via
  `POST /api/send`; functionality and the server's authority are unchanged.

## Recommendation

Reimplement mosh-style **predictive local echo, mobile-first** (openspec change
`mobile-predictive-echo`). Defer: `attach` predictive echo (needs a client VT model),
`attach` reconnect/resync roaming (borrow the heartbeat idea), and any SSP adoption (not
worth it — mobile already has the screen-state model, attach's backpressure suffices).
Do **not** attempt to adopt mosh itself (UDP + GPL).
