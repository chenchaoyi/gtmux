# mobile-send-receipt-first — the phone's send confirms by the agent's receipt, not by scraping its TUI

## Why

A message sent from the mobile app (`POST /api/send`) is DELIVERED by tmux
(`paste-buffer` + Enter) but CONFIRMED by scraping the pane's rendered screen
(`dispatch.PasteAndSubmit` → `confirmPaste` → `SplitInputRegion`). That scrape is coupled
to each agent's TUI, and it has accumulated per-agent special cases — Claude's titled
input border, opencode's left-edge box, Codex's footer-rendered working state. Every new
agent adds more quirks and every agent version can drift; the symptom is a send that
false-fails with **"the input box didn't confirm the full message"**, intermittently.

A 2026-08 report: sending into a **working Codex** pane failed "时好时坏", and the message
never reached Codex. The immediate cause of that case is fixed separately (the busy
heuristic watched only the transcript ABOVE the box and missed Codex's footer timer — see
change `dispatch-busy-detection` / the whole-frame-motion fix). But that is one more patch
on a brittle surface. The architectural problem is: **the mobile path relies solely on the
screen-scrape and never consults the deterministic channel that already exists.**

gtmux already has a better signal. When an agent submits a prompt it fires a hook —
`UserPromptSubmit` — recorded on the event stream and carrying the prompt's normalized
HEAD. The CLI's `Deliver` prefers this receipt (deterministic, agent-declared,
screen-independent) and falls back to the scrape. But `POST /api/send` was deliberately
built to use ONLY the scrape (`PasteAndSubmit`), skipping the receipt wait, so the phone
returns a fast input echo — routing it through `Deliver` regressed the echo by a poll
cycle (`internal/app/serve.go`, the "FAST ECHO" comment). The bet was "the pre-submit
paste guard confirms the draft reliably"; it holds for Claude and breaks for any agent
whose composer the scrape can't read while it works.

Layer summary, to keep the vocabulary straight:

- **Delivery** = 100% tmux (`send-keys` / `paste-buffer`). tmux confirms only that bytes
  reached the pty — NOT that the program accepted or submitted them.
- **Confirmation** = a cooperation: ① the agent's `UserPromptSubmit` RECEIPT (the only
  signal that "the agent received & submitted MY text", matched on head) and ② the tmux
  screen-scrape (a TUI-coupled proxy). `Deliver` uses ①+②; `PasteAndSubmit` uses ② only.

## What changes

Decouple **sent** from **confirmed**, so the phone stays fast AND confirmation stops
depending on TUI analysis:

1. **Optimistic delivery, unchanged speed.** Paste + submit best-effort and return with
   the input echo, as today. The screen-scrape is DEMOTED to a pre-submit sanity check
   (drop copy-mode, did the paste land) — no longer the arbiter of "did it send".
2. **Reconcile with the agent's receipt.** Confirmation comes from the `UserPromptSubmit`
   receipt matched on the prompt head — portable across every hook-equipped agent
   (Claude, Codex, opencode…), no per-TUI code. A matching receipt within a grace window
   confirms; none ⇒ the phone surfaces "might not have sent", instead of deriving failure
   from a single brittle scrape frame.
3. **A working pane never hard-fails synchronously.** When the pane is busy (radar state
   or observed whole-frame motion), an unconfirmed-but-placed paste submits best-effort;
   a message that still can't be confirmed is HELD and re-delivered when the pane goes
   idle (radar's working→idle transition), rather than dropped with a red banner. This is
   the safe answer whether the agent accepts input mid-turn (best-effort lands it) or
   rejects it (queue-until-idle lands it) — no need to know which per agent.
4. **Per-agent receipt reliability becomes Tier-1.** For an agent whose submit receipt is
   not yet wired or is unreliable, THAT is the thing to fix in its driver/manifest (see
   `internal/driver`, `internal/agents`) — the scrape is a shrinking last resort. This is
   the agent-drivers direction: tmux is the UI, the per-agent receipt is the source of
   truth for "landed".

Non-agent (plain shell) panes have no receipt and no structured input box; they stay
fire-and-forget best-effort — tmux delivered the bytes and there is no app-level ack to
get. This is already the behavior (`structured == false ⇒ pasteUnverifiable`); the change
only makes it explicit and consistent with the tiers above.

## Open questions (to settle during implementation)

- Grace window for the receipt reconcile, and how the phone renders the three states
  (sent / pending / could-not-confirm) without a jarring banner.
- Where the held (queue-until-idle) message lives and its idempotency key, so a retry or a
  reconnect never double-delivers.
- Whether `POST /api/send` gains an async "pending id" in its response contract, or the
  reconcile rides the existing transcript/event poll the app already runs.
