# Design — hq-capture-loop (the mechanism spec)

This is the mechanism manual for the iteration. For **each** layer it states: the
trigger condition, the data flow, where it lands on disk, the signal mark, the config
defaults (K / N), and **why it sits at that layer** (cheap → thorough). It closes with
the consult half-loop and the welded board/KB definitions.

> **Commander's tuning (2026-07-19), folded in below:** mandatory capture is scoped to
> `correction` / `crash` / `recurrence` only (NOT `done`/`resolved` — those would degrade
> into ritual noise and manufacture filler entries); `gtmux capture` is a PUBLIC command
> so any worker can feed the spool; the dispatch-time KB echo is promoted to a
> first-class deliverable (not a PR3 stretch); the distill auto-triggers (③) are deferred
> behind an observation gate — ship ① + ② first, run manual distill for a while, and only
> add density/correction auto-firing if capture still slips; `K`/`N` are config (default
> N = 5, K = 10, range 10–12); spool entries carry a dedup key + topic tag so distill
> merges into an existing entry instead of making near-duplicates.

## 0. The core problem, restated as a control question

HQ's perception loop is reliable because it is **forced**: the wake protocol
(`internal/hqwake`, spec `hq-wake-protocol`) types a `» gtmux·<class>` signal line for
every decision-dense event, so the turn *cannot* end without HQ judging that event. The
loop shape is `SENSE → JUDGE → REPORT`.

Capture has **no such forcing function.** It is reached only if HQ, mid-triage, chooses
to switch context and write the KB. Empirically it does not. The board makes this worse:
`board.md` is in-loop (part of REPORT) and gets updated reflexively, so HQ feels it has
"remembered" when it has only recorded an ephemeral posture — the KB stays empty.

The design question is therefore **not** "how do we remind HQ to capture" (we already
know self-discipline breaks). It is: **how do we make the events most likely to carry a
durable lesson impossible to close without a capture verdict, without forcing capture so
broadly that it degrades into ritual filler; and how do we make noticing cheap enough,
for the whole fleet, that being busy is no longer an excuse.** The three layers answer
that at three costs.

```
        ┌─────────────────────── the loop today ───────────────────────┐
        │  SENSE ──▶ JUDGE ──▶ REPORT                                   │
        │              │                                               │
        │              └──▶ (capture)  ← optional, out-of-loop, breaks │
        └──────────────────────────────────────────────────────────────┘

        ┌─────────────────────── the loop after ───────────────────────┐
        │  SENSE ──▶ JUDGE ──▶ CAPTURE? ──▶ REPORT                      │
        │                        │  mandatory verdict on               │
        │                        │  correction / crash / recurrence    │
        │                        │  (done/resolved: opportunistic,      │
        │                        │   silent default — no forced verdict)│
        │  ② gtmux capture ───────┘  any worker, cheap notice → spool   │
        │  ③ density/correction trigger ──▶ distill pass  (deferred)    │
        └──────────────────────────────────────────────────────────────┘
```

---

## Layer ① — Protocol: the capture-verify (playbook only, zero engineering)

**What it is.** Upgrade the closed-loop turn from `SENSE → JUDGE → REPORT` to `SENSE →
JUDGE → CAPTURE? → REPORT`, and make the CAPTURE? step **produce a verdict that is part
of the turn's visible output** — but only for the events that reliably carry a durable
lesson, so the forcing stays high-signal.

**Trigger condition — which closures FORCE a verdict.** Exactly three classes, chosen
because each is a durable lesson almost by definition:

| class | why it forces a capture verdict |
|---|---|
| `correction` (commander corrects HQ) | a first-class lesson by definition — the highest-value source; also the ③ trigger |
| `crash` / `StopFailure` | a footgun paid for — belongs in `pitfalls.md` |
| `recurrence` (any footgun/fact hit a **second** time) | a repeat is proof the fact is cross-cutting and was NOT yet captured — the exact miss this change targets |

**`done` / `resolved` are NOT forced.** For those, capture is **opportunistic with a
silent default**: if a genuinely reusable fact surfaced, HQ captures it and marks it;
otherwise it says nothing and moves on. Forcing a verdict on these high-frequency
closures would degrade into ritual noise and pressure HQ into manufacturing filler KB
entries — the opposite of a curated base. Routine intermediate steps, `tick`,
`new-session`, `waiting` are likewise never forced.

**The verdict (on a forced class, exactly one, in the turn):**

- `⟣ 📓 captured: <topic-file>` — this turn wrote/updated a KB entry, naming which topic
  file it landed in (`accounts` | `workflows` | `best-practices` | `pitfalls` |
  `corrections`); **or**
- an explicit **"nothing durable"** clause — one clause saying *why* this closure is not
  a reusable, cross-cutting fact (e.g. "one-off in this repo, board-only").

**The judgment criterion (what counts as capturable):** `reusable ∧ cross-cutting
(across sessions / repos / tasks) ∧ not unique to this conversation`. If it matches
account / procedure / workflow / pitfall / correction → write the KB. Pure this-task
state (who is doing what, a specific PR number) is **not** KB — it goes to the board.

**Data flow.** Entirely inside the HQ turn: JUDGE already reads the wake line + pulled
delta; CAPTURE? adds a single classification (durable? which topic?) and either a
write-own-notes action into `knowledge/<topic>.md` (HQ's existing authority) or the
"nothing durable" clause. REPORT then carries the `⟣ 📓` line **only when a real capture
happened** (or the ▪/✅/⚠ line already planned).

**Landing place.** `~/.config/gtmux/hq/knowledge/<topic>.md`, using HQ's existing
own-notes write authority — no new mechanism. Prefer UPDATE over append (dedup).

**Signal mark.** New register glyph `⟣ 📓` = *captured*, added to the signal-register
vocabulary alongside `✅ ▪ ◈ ⚠`. It is emitted **only on a real capture** — never as an
empty "I considered it" marker — and follows every existing register rule (one line,
signal-domain only, never mixed with human prose).

**Config defaults.** None — this layer is pure playbook text. It ships by bumping
`hqPlaybookVersion` 7 → 8 and editing `hqInstructions`; existing homes pick it up on
their next managed-playbook upgrade.

**Why this layer is first (cheapest → most effective per unit cost).** It is zero
engineering and takes effect immediately on the next `gtmux hq` upgrade. It converts
"remember to capture" into "you cannot close a correction / crash / recurrence without a
capture verdict" — the same forcing trick the wake protocol already uses for perception,
now applied to capture at exactly the three points where a lesson is near-certain. This
alone recovers most of the lost value; ② removes the remaining friction, and ③ (later,
gated) closes the timing gap only if it proves necessary.

---

## Layer ② — Tool: `gtmux capture` makes *noticing* cheap, for the whole fleet (new PUBLIC command)

**What it is.** A one-line CLI that records a distill *candidate* the instant anyone
notices something, decoupling **noticing** (cheap, in-the-moment) from **writing it up
well** (batched, at distill time). Writing a full polished KB entry mid-work is too
expensive and gets skipped; a candidate line is not.

**Interface.**

```
gtmux capture "<one-line lesson> @<topic>"      # topic ∈ accounts|workflows|best-practices|pitfalls|corrections
gtmux capture --list                            # show the pending-distill queue
```

**Why PUBLIC, not HQ-only (commander's call, and why it is safe).** Making `capture` a
public command widens the capture surface from **one HQ to the whole fleet**: any worker
that learns a cross-cutting fact (a footgun in a repo, an account procedure, a workflow
quirk) can drop it into the spool in one line, at the moment of learning, without routing
it through HQ. This is safe because **a candidate is not a KB entry**: the spool is a
raw inbox, and the distill pass (HQ's curation) is the quality gate that decides what is
durable, folds it into the right topic, dedups, and prunes. Opening the *input* while
keeping *curation* centralized has no downside — worst case a candidate is dropped at
distill time — and it directly attacks the root cause (capture depended on a single busy
actor remembering to do it).

**Trigger condition.** Human-invoked by HQ or any worker, any time a durable fact is
noticed but distilling it now is too expensive — including *as* the ① verdict when the
lesson is real but not yet worth a full entry (capture now, distill later). Not
gtmux-raised; it is a tool anyone reaches for.

**Data flow.** `gtmux capture` appends **one JSON line** — the lesson text + topic + a
**dedup key** + **auto-collected event context** — to the pending-distill spool.
Auto-collected context (so distill has provenance without re-typing): current/related
`pane_id`, the current event `seq`, `task_id` if any, a timestamp. The **dedup key**
(topic + a slug of the lesson, or an explicit key) lets the distill pass MERGE a candidate
into an existing KB entry or an earlier same-key candidate instead of manufacturing a
near-duplicate — the exact failure the current KB shows, where send-reliability got
scattered across three separate entries for want of a merge key. The distill pass (③, or
the manual/periodic pass until then) drains the spool, folding each candidate into the
right KB topic keyed by (topic, dedup key), then truncates it.

**Landing place.** `~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` (or the
state-dir equivalent under `state.HQHome()` / `state.Dir()`), one JSON object per line —
append-only, drained + truncated by the distill pass. A dot-prefixed file so it does not
clutter the curated KB topic list.

**Line shape (illustrative — note `topic` tag + `key`):**

```json
{"at": 1721390400, "topic": "pitfalls", "key": "pitfalls/wrangler-office-tls-reset", "lesson": "wrangler TLS-resets from the office; retry", "pane": "%12", "seq": 8412, "task": "t-abc"}
```

**Signal mark.** None of its own — the *outcome* of a distill drain is briefed with the
existing distill one-liner; a bare `gtmux capture` at notice-time prints only a terse
confirmation. When HQ itself captures as an ① verdict, that is where `⟣ 📓` appears.

**Config defaults.** `N` (spool depth that forces a distill) is config — see ③; default
**N = 5**. No other config here.

**Why this layer is second.** ① makes capture mandatory at three points but still asks
the actor to *write* at the busy moment, and only covers HQ. ② removes both limits:
"notice cheaply now, write well later," for anyone. It is a small, self-contained
increment (one command + one append-only file with a merge key) that de-risks ① under
load and multiplies the capture surface — but ① already delivers value without it, hence
second.

---

## Consult half-loop — so capture is not wasted (dispatch-KB echo, promoted to first-class)

Capture is only half the value chain: `capture → consult → suggest`. If knowledge is
captured but never consulted, the KB is write-only and the sharper-answer payoff never
lands. Two mechanisms, and the tool one is now a **first-class deliverable, not a
stretch**:

- **Protocol (playbook, ships in PR1 with ①):** before **advising or dispatching**, HQ
  MUST first **consult the relevant KB topic** — hardened from the existing soft "check
  the relevant topic" into a **hard precondition**. When it advises, HQ should name the
  KB entry its advice rests on; if **no KB covers** the case, that gap is itself a
  capture trigger — HQ captures it afterward.
- **Tool echo — the dispatch-time KB echo (first-class, its own PR, right after ②):** at
  `gtmux spawn` / dispatch, auto-echo the **pitfalls / workflows summary** matching the
  target repo (by cwd repo name) and the goal keywords, handing it to the worker as part
  of the launch. This is **the only mechanism that structurally closes "captured but
  never used"** — it surfaces the KB at the exact moment work starts, as a tool
  guarantee rather than an HQ-discipline hope. That is why the commander promoted it out
  of the ③ bucket: it is the payoff of ① + ②, and must not wait behind the deferred
  auto-triggers.

Consult closes the loop: ① / ② fill the KB; the hard precondition + the dispatch echo
spend it.

---

## Layer ③ — Mechanism: distill fires on density / correction (DEFERRED behind an observation gate)

**What it is today.** `internal/app/distill.go` raises a `[CONTROL gtmux:distill]`
record from the serve slow-tick, gated by a **coarse cadence**: a rate limit
(`distillMinInterval`, ≥1/day), then a VOLUME floor (`distillVolumeFloor`, sized to the
event-log rotation), OR a WEEKLY floor (`distillWeeklyFloor`), with a ZERO-CHANGE gate
(nothing notable accrued → no fire, no cost). It advances a `last-distill` watermark
(event seq + timestamp) so each pass distills only the DELTA. **This is purely periodic.**

**Why this is DEFERRED (commander's staging).** Do **not** build all three layers at
once. Ship ① + ② + the dispatch echo, then run distillation on the **existing manual /
periodic** trigger for a while and **observe**: with capture now mandatory at three
points and cheap for the whole fleet, does the KB actually stay current? If it does, the
auto-triggers are unnecessary complexity. **Only if capture still slips** — the spool
grows but the periodic pass is too infrequent to keep up — do we add the event-driven
firing. This keeps the timing sensor untouched until evidence demands it.

**What ③ adds WHEN built (the target behavior).** A `distill` trigger fires when **any**
of, layered on top of today's rate limit + zero-change gate + periodic floor:

| trigger | condition | default (config) |
|---|---|---|
| **density** | ≥ K notable *closures* accrued since the last distill watermark | **K = 10** (config; range 10–12) |
| **correction** | any `correction`-class event since the watermark | immediate (still obeys the min interval) |
| **spool depth** | the pending-distill spool (②) has ≥ N entries | **N = 5** (config) |
| **periodic floor** | the existing weekly/volume cadence | unchanged (retained lower bound) |

The existing rate limit (`distillMinInterval`) and ZERO-CHANGE gate still apply *first*,
so the new triggers can never busy-loop the pane. The correction trigger distills
promptly but must still respect the minimum interval so a correction storm cannot hammer
the pane.

**Data flow (when built).** Unchanged plumbing. The slow-tick sensor already reads the
event delta since the watermark and counts notable events; this adds (a) counting
*closures* for the density trigger, (b) checking for a `correction`-class event, and (c) a
cheap line-count of the spool for the depth trigger. On fire it emits the same
`hqfeed.ControlDistill` record on the silent feed (like `self-check` — a low-urgency
maintenance signal, NOT a typed wake line that interrupts the pane) and advances the
watermark exactly as today.

**The distill pass itself (HQ side, playbook `Iterate` ritual) — runs from PR2 onward,
manual/periodic until ③.** Drain `.pending-distill.jsonl` → for **each** candidate, MERGE
by (topic, dedup key) into the matching KB entry in preference to appending → prune stale
/ dead entries → truncate the spool → emit a **one-line brief only on real curation**
(silent otherwise). It works the DELTA (watermark-bounded), so it consolidates rather
than re-summarizing, and never duplicates what ① / ② already wrote.

**Config defaults.** `K = 10` (density, config, range 10–12), `N = 5` (spool depth,
config). Both surfaced as configuration (tunable WITHOUT a release) next to the existing
`distill*` values. Provisional — tune against real fleet cadence.

**Signal mark.** The existing distill brief (a one-liner on real curation); no new glyph.

**Why this layer is last (root-cause, most engineering, and gated).** ① and ② make
*individual* captures happen; ③ only fixes **when the batch consolidation runs**. Because
① + ② may already keep the KB current, ③ is built only after observation proves capture
still slips — the most invasive change, deferred until it earns its place.

---

## Board vs knowledge base — welded definitions (the anti-confusion clause)

The root failure was HQ treating a board note as if it were capture. One clause is welded
into the charter to make that impossible:

- **Situation board (`board.md`) = HQ's EPHEMERAL private posture.** mode/source,
  priority, health, pending decision, standing context. gtmux does **not** read it back;
  HQ re-reads it itself after a `/compact` or context reset. It is per-fleet-moment state.
- **Knowledge base (`knowledge/`) = the MACHINE's durable, cross-session, reusable
  memory.** account / workflow / best-practice / pitfall / correction.
- **The capture-verify can ONLY route a lesson into the KB.** "I noted the board" can
  **never** count as a capture. The two may be written together (board records posture,
  KB records the reusable fact) but **neither substitutes for the other.**

This is why ①'s verdict names a *topic file* (`⟣ 📓 captured: <topic-file>`): the mark
itself asserts the lesson reached the KB, not the board.

---

## Reuse map (do not rebuild)

| need | existing mechanism to reuse |
|---|---|
| distill control record + delivery | `hqfeed.ControlDistill` on the silent feed (`internal/hqfeed`) |
| distill timing sensor + watermark | `internal/app/distill.go` (`distillSensor`, `readDistillMark`/`writeDistillMark`, `last-distill`) |
| severity/closure classification of the delta | `internal/events` (`SeverityRank`, `SevNotable`; closure/correction classes) |
| HQ own-notes write authority | existing `knowledge/` write path — no new permission |
| playbook versioning + auto-upgrade | `hqPlaybookVersion` + managed-AGENTS.md regen (spec `supervisor-agent`) |
| CLI dispatch + docs-drift gate | `internal/app/app.go` command registry, `check-design.sh`, `docs/cli.md`, `help.go` |

## Open questions / tuning

- **K and N are provisional** (K = 10, range 10–12; N = 5), and only matter once ③ is
  built. Tune against a real active fleet: too low → distill churn; too high → the
  density trigger rarely beats the periodic floor.
- **The observation gate for ③** needs a concrete "still slipping" signal — e.g. spool
  depth persistently above N between periodic passes, or the commander flagging another
  capture miss. Decide the trip condition before building ③.
- **Dedup-key granularity.** Topic + lesson-slug is the default; confirm it merges the
  scattered send-reliability entries without over-merging distinct facts.
- **`gtmux capture` is PUBLIC** (`gtmux --help`, `docs/cli.md`) — not a HIDDEN-allowlist
  entry — so the whole fleet can feed the spool (per the commander's call above).
