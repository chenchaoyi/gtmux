# Echo audit — the instrumented numbers behind C14/C7 (2026-08-08)

Task 1 of this change, run against the live stream on the commander's Mac
(`~/.local/share/gtmux/events.jsonl`, 12 852 records, 2026-07-12 → 2026-08-08).
Aggregates only — no event content is reproduced here.

**Audit window:** seq 7039 → 11488 (2026-08-02 10:37 → 2026-08-08 23:51), 4 451 records.
The window starts at the first `unread` knock ever observed in the stream, so every number
below describes the mechanism as shipped in #650.

## 1.1 — Did the 2026-08-04 loop predate #650 on the running serve? NO.

The first `unread` knock in the stream is at **seq 7039, 2026-08-02 10:37** — two days
BEFORE the cited loop (cursors 8439→8450, 2026-08-04 12:56–13:04). A knock cannot be
delivered by a serve that does not have the sensor, so the resident serve was already
running #650 code. **The residual is real, not a pre-#650 artifact.**

Re-reading the cited window confirms the proposal's own correction. HQ was pane `%4`
(it has authored every wake echo since 2026-07-30); `%21` was a concurrently running
user-direct session:

| seq | pane | note |
|---|---|---|
| 8435–8437 | `%21` | SessionStart + two prompt submissions — countable |
| 8438–8441 | `%4` | HQ's own turn — excluded from the count, still returned by the pull |
| 8442, 8444, 8449 | `%21` | turn-ends and a submission — countable |
| 8443, 8445–8448, 8450–8455 | `%4` | HQ's own turns |

So the four knocks were **countable and correct**. The pane filter did its job.

## 1.2 — The ~70 % echo figure, instrumented

The stream does not record what a wake batch contained: HQ's wake-driven prompts carry only
the batch id (`#a43f13`) as their summary — the driver receipt from `hq-wake-reliability`.
The recoverable record is HQ's own **turn-end summaries**, which quote the line HQ was
answering. 88 quoted `unread` lines → **55 distinct (count, cursor) knocks** over 6.6 days.

**Knock volume is not the problem.**

| unconsumed count in the line | 1 | 2 | 3 | 4 | 6 | 19 |
|---|---|---|---|---|---|---|
| knocks | 31 | 13 | 3 | 6 | 1 | 1 |

Median **1**, mean 2.1, ≈ **8 knocks/day**. 44/55 (80 %) of knocks are about ≤ 2 events.

**The pull those knocks demand is what is 70 % echo.** Replaying what
`gtmux events --since-seq <cursor>` returned at each knock (53 knocks with a non-empty
window, 300 records total):

| composition of the pulled records | count | share |
|---|---|---|
| HQ's OWN records (excluded from the count, still returned by the read) | 206 | **68.7 %** |
| countable — the events the knock was actually about | 94 | 31.3 % |
| empty-pane lifecycle blinks | 0 | 0.0 % |

For the dominant `≤2 unconsumed` case (42/53 knocks): 181 records pulled, **136 of them
HQ's own = 75.1 % echo**.

**Verdict on the ~70 % estimate: the number is right, its subject was not.** The knock is
not fed by HQ's echo — the pane exclusion works, and blinks contributed nothing to these
windows. HQ's *read* is drowned in it: a median knock asks HQ to spend a turn reading ~3.4
records to find **1** new fact. Across the whole window HQ's own records are 2 908 / 4 451
= **65 % of the stream**.

**Root cause of that 68.7 %: the count and the read disagree about what HQ owes.**
`unreadCount` excludes HQ's own pane (`internal/hq/unread.go:121`); `gtmux events
--since-seq` returns everything (`internal/hq/eventscmd.go:153-165`). One set defines the
debt, a different, larger set is what HQ must read to clear it.

**Class-wake coverage of the debt** (the double-knock question's raw material) — 1 543
countable records, classified by whether any wake class could have claimed them:

| would a class have claimed it? | records | share |
|---|---|---|
| `done` — *only if the pane was awaited/unattended* | 581 | 37.7 % |
| `goal-changed` (`origin:"instruction"`) | 405 | 26.2 % |
| **no class claims it** | 336 | **21.8 %** |
| `new-session` | 99 | 6.4 % |
| `waiting`/`asks` | 82 | 5.3 % |
| `asks` (turn-end classified asking) | 40 | 2.6 % |

78.2 % is an **upper bound**, not a measurement of double-knocks: `done` is conditional on
the pane being awaited, and no wake class fires at all for a pane-less record (see 1.3).

## 1.3 — Verdict: an ack'd-wake-delivered event is NOT consumption-equivalent

**Decision: NO.** Four reasons, in the order the evidence produced them.

1. **It attacks the wrong quantity.** The measured cost is not knock volume (8/day, median
   1 event) but pull composition (68.7 % echo). Making delivered events consumption-
   equivalent would remove knocks HQ is not complaining about and leave the read HQ *is*
   complaining about exactly as noisy.
2. **The price is the guarantee itself.** Up to 78 % of the debt is class-eligible, while
   the 21.8 % that no class claims is precisely the population this net exists for — the
   2026-08-01 unclaimed turn-end that `unread.go`'s own header commemorates.
3. **A wake line is a summary, not the record.** `done`/`goal-changed` lines carry a
   clamped head, not the reply or the prompt. Marking a record read because a summary of it
   was delivered is the playbook's "a filter is a triage shortcut, never your model of the
   world" error, moved one layer down into the mechanism.
4. **Decisive: pane-less records can never be delivered as a class wake.** The entire wake
   channel is gated on `if pane != ""` (`internal/hook/hook.go:679`). For a native
   (non-tmux) session and for gtmux's own control records, the `unread` knock is the ONLY
   channel that exists. Consumption-equivalence would not touch them today, but it
   establishes "delivered ⇒ read" as a principle, and this is the population that principle
   would eventually blind.

**What the audit says to do instead:** make the read's exclusion set equal the count's
(the 68.7 %), and give the knock enough composition that HQ can often judge without pulling
at all. The first is a scope question for the commander — recorded in the proposal under
"Audit-driven addition", NOT implemented here.

## C7 correction — `tasks.md` 2.1's literal rule would have caused a regression

2.1 said "skip records with an empty `pane`". Measured against the window, empty-pane
records are **not** all process blinks:

| empty-pane family | records | what it is |
|---|---|---|
| `SessionStart`/`SessionEnd` | 50 | the blink — *and* real native sessions (below) |
| `Stop`/`UserPromptSubmit` | 39 | **real agent turns** — Claude Code 27, Codex 12; growing (22 on 08-07) |
| `gtmux:self-check` ×8, `gtmux:distill` ×1 | 9 | **the maintenance triggers** #647 shipped so they would reach HQ |

The blunt rule would have stopped counting all 98 — silently un-shipping #647 and blinding
HQ to every pane-less agent's activity, which (per 1.3 §4) has no other channel. The
existing `TestUnreadCountsControlRecords` would have failed; the native-turn half would
have passed CI unnoticed.

**The `session` field cannot be the discriminator either.** `session` is the tmux
*session_name*, set only when a pane exists (`internal/hook/hook.go:526-529`), so EVERY
pane-less record has an empty session by construction. The spec delta's "no pane and no
session" describes the measurement, not a usable rule.

**Even "pane-less lifecycle events" over-reaches:** 13 of 28 pane-less `SessionStart`s are
followed by real pane-less turns from the same agent within 30 min — they are live native
sessions coming online, not blinks.

**Corrected criterion — liveness pairing.** Exclude a pane-less `SessionStart` only when a
pane-less `SessionEnd` from the same agent lands within ≤ 10 s, and exclude that end with
it. Measured on this window:

| rule | records excluded | share of debt | solo knocks prevented (6.6 d) |
|---|---|---|---|
| literal "any empty pane" (2.1 as written) | 98 | 6.4 % | — (regressive) |
| "any pane-less lifecycle event" | 50 | 3.2 % | ≤ 12 |
| **liveness pairing (adopted)** | **30** | **1.9 %** | **≤ 8** |

A pairing window of 5 s and one of 60 s catch the same 15/28 starts, so 10 s is not a tuned constant —
it is the flat part of the curve. **C7's real size is ~2 % of the debt.** It is worth
fixing because it is noise HQ can never act on, not because it is large; the proposal's
framing of it as a major noise source is corrected by this audit.
