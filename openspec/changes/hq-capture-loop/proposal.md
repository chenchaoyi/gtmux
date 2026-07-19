# hq-capture-loop — weld knowledge capture into HQ's loop

## Why

HQ's **perception loop** (WAKE → PULL → JUDGE → REPORT) is driven by the wake
protocol: every decision-dense event types a signal line into the pane, so each event
is forced to a close — nothing is silently dropped. That machinery is why HQ reliably
triages, dispatches, and reports.

Knowledge **capture sits OUTSIDE that loop.** It is a "conscience action" with no
enforced trigger point. The moment HQ is busy triaging / dispatching / briefing, capture
vanishes entirely — and a recent commander correction proved it: HQ kept the situation
board meticulously current for a whole session yet folded **zero** durable facts into the
knowledge base. That is not a discipline lapse to scold; it is the predictable outcome of
leaving the single most valuable step un-mechanized.

Two structural reasons it always breaks:

1. **The board is in the loop; the KB is not.** `board.md` is right at hand and is part
   of REPORT, so HQ updates it reflexively. The KB is a separate, deliberate context
   switch. HQ then comforts itself with "I wrote the board" and mistakes an ephemeral
   posture note for durable distillation.
2. **A chain is only as strong as its weakest link:** capture → consult → suggest. If
   capture breaks, "give a sharper answer grounded in the KB" has no source. Yet the
   playbook itself calls the knowledge base *"your single most important job."* The loop
   that produces HQ's core value is the one with no forcing function.

## What changes

Make **CAPTURE a first-class, non-skippable step of the loop** instead of an
out-of-loop good intention, and make the **consult half-loop** actually spend what was
saved. Three layers, cheapest → most thorough, plus a definition weld:

- **① Protocol layer (playbook only, zero engineering):** upgrade the closed-loop turn
  from `SENSE → JUDGE → REPORT` to `SENSE → JUDGE → CAPTURE? → REPORT`. A **capture
  verdict is mandatory ONLY on `correction` / `crash` / `recurrence`** (any footgun/fact
  hit a second time) — the turn must emit either `⟣ 📓 captured: <topic-file>` or an
  explicit "nothing durable" clause. `done` / `resolved` are **opportunistic, silent by
  default** (forcing them would breed ritual filler). A new `⟣ 📓` glyph, emitted only on
  a real capture. Consult is hardened into a **hard precondition** before
  advising/dispatching. The board-vs-KB definitions are **welded** into the charter so "I
  noted the board" can never stand in for capture.
- **② Tool layer (`gtmux capture`, new PUBLIC command):** make *noticing* cheap for the
  **whole fleet** — any worker, not just HQ. One line —
  `gtmux capture "<lesson> @<topic>"` — appends a candidate plus a **dedup key + topic
  tag** and auto-collected event context to a `pending-distill` spool. A candidate is not
  a KB entry; HQ's distill pass is the quality gate, so opening the input is safe. The
  dedup key lets distill MERGE into an existing entry instead of scattering near-duplicates.
- **Consult echo (tool layer, first-class):** at `gtmux spawn` / dispatch, auto-echo the
  KB (pitfalls/workflows) matching the target repo/goal to the worker. This is **the only
  mechanism that structurally closes "captured but never used"**, so it is promoted to a
  first-class deliverable landing right after ②.
- **③ Mechanism layer (distill triggers) — DEFERRED behind an observation gate:** today's
  `distill` control record is purely periodic. Rather than build it up front, ship ① + ②
  and run distillation manually/periodically for a while; **only if capture still slips**
  add the event-driven firing (K accumulated notable closures / any correction / spool ≥
  N), with the periodic floor kept as the lower bound. `K`/`N` are config (default N = 5,
  K = 10, range 10–12).

## Impact

- **Affected spec:** `supervisor-agent` (playbook rituals, the distill trigger, the new
  `gtmux capture` command, consult precondition, board/KB weld).
- **Playbook version:** `hqPlaybookVersion` 7 → 8. Any change to `hqInstructions` MUST
  bump it, so existing HQ homes adopt the capture-loop on their next managed-playbook
  upgrade (the situation board + knowledge base are untouched by the upgrade).
- **New CLI surface:** `gtmux capture` (PUBLIC — fleet-wide, not HQ-only) — documented in
  the CLAUDE.md command list, `gtmux --help` (en+zh), and `docs/cli.md`, per the CLI-drift
  rule; NOT the `check-design.sh` HIDDEN allowlist.
- **New state:** a `pending-distill` spool under the HQ home
  (`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` or the state equivalent), each
  line carrying a dedup key + topic tag so distill merges instead of duplicating.
- **Reuse, don't rebuild:** the distill watermark (`last-distill`), the `[CONTROL
  gtmux:distill]` control-record pipeline (`internal/app/distill.go`,
  `internal/hqfeed`), and the severity-tagged event ledger already exist — this change
  extends the distill *trigger conditions* and adds a spool; it does not re-plumb the
  timing loop or the feed.

## Rollout — one change, staged so ① + ② land first and ③ is gated

Do **not** build all three layers at once (commander's call). Ship the protocol + the
cheap-notice tool + the consult echo, observe, and add the auto-triggers only if capture
still slips (see `tasks.md` for the task groups):

- **PR1 — group A (pure playbook v8, zero engineering risk, ships today):** ① protocol
  (the `⟣ 📓` glyph + mandatory capture verdict on correction/crash/recurrence, done/
  resolved opportunistic-silent), consult hard-precondition, board/KB weld.
- **PR2 — group B (engineering increment, right after):** the `gtmux capture` PUBLIC
  command + `pending-distill` spool with a dedup key + topic tag (②); the distill pass
  drains + merges the spool on the existing manual/periodic trigger.
- **PR3 — group C (engineering increment, first-class):** the dispatch-time KB echo — the
  tool-layer mechanism that closes "captured but never used."
- **PR4 — group D (DEFERRED behind the observation gate):** the ③ distill auto-triggers
  (density / correction / spool), built only if observation shows capture still slips;
  may intentionally never ship.
