# Design: hq-knowledge-distillation

> Investigation deliverable. Evaluates the four dimensions the commander asked about
> (cadence · trigger · dedup vs event-capture · playbook seeding) against the existing
> code, and recommends a concrete shape. Not implemented until approved.

## What already exists (so we build on it, not beside it)

| Mechanism | Code | Purpose | Cadence |
|---|---|---|---|
| Event-driven **capture** | playbook `hqInstructions` Knowledge-base § | write a durable fact the MOMENT it's learned | continuous, per-fact |
| **self-check** sensor | `internal/app/selfcheck.go` → `[CONTROL gtmux:self-check]` | HQ reviews its OWN artifacts' HEALTH (ledger archival, feed/memory health), cleans silently | ≤1/h; idle / threshold / daily floor |
| **tick** | `hqwake` / slow-tick | the user-facing periodic summary brief | 10 min / burst 5, zero-change gate |
| Correction→charter loop | `knowledge/corrections.md` | distil a lesson when corrected / a footgun repeats | event-driven |

The gap: the playbook's **"Iterate: periodically review — prune what's dead, merge
duplicates"** has no trigger. Everything above either captures single facts as they
happen, checks HQ's own housekeeping health, or reports — none does the retrospective
*fleet-activity → KB consolidation + prune* on a clock.

The `self-check` sensor is the exact template for how to add a deterministic,
LLM-free, context-reset-proof trigger: a cheap rate gate, a last-run marker, a pure
`shouldX(...)` decision (unit-testable without tmux/disk), `EmitControl` into the
feed, HQ acts on the control record. We follow it.

## D1 — Cadence: weekly floor + an event-volume floor (whichever first), zero-change gate

- **Weekly** is the right *baseline* for consolidate-and-prune: KB curation is not a
  daily need, and moment-capture already handles same-day durable facts. A daily
  heavy pass would mostly re-scan low-signal churn and overlap moment-capture.
- **But events retention is SIZE-based** (`internal/events/events.go`: 20 MB cap,
  ~×2 rotated), so a busy fleet's log rotates in less than a week. If distill only
  fired weekly, a heavy week would lose its early events to rotation before they were
  ever distilled. So add an **event-volume floor**: distill also fires once ~N new
  events have accrued since the last watermark (N tuned to stay comfortably inside one
  rotated generation). Quiet fleet → weekly; busy fleet → sooner, before rotation.
- **Zero-change gate** (like `tick`): if nothing notable happened since the last
  distill, skip — no wake, no cost.
- Precedence mirrors `shouldSelfCheck`: rate-limit gate → volume floor (busy) →
  weekly floor (time) → else skip.

*Recommendation: weekly floor + event-volume floor + zero-change gate. Daily is
explicitly NOT recommended (noise + overlap with moment-capture).*

## D2 — Trigger: an INDEPENDENT sensor, reusing the self-check infrastructure

Reusing `self-check` itself is wrong: it is HOURLY-capable HEALTH housekeeping told
to "clean silently"; folding a weekly heavy retrospective distill into it would muddy
its purpose and fire at the wrong cadence. Reusing `tick` is wrong: it's the
user-facing brief, not a KB write.

*Recommendation:* a **new `distillSensor(now)`** beside `selfCheckSensor`, emitting a
new `ControlDistill` = `gtmux:distill` record, with its own `last-distill` marker and
`shouldDistill(...)` pure decision. It reuses the *pattern and plumbing* (slow-tick
caller, `hqfeed.EmitControl`, a new `hqwake` class, `hqfeedcmd` rendering) but is its
own trigger. This is precisely how `self-check` was itself added — consistent,
low-risk, independently testable.

## D3 — Dedup vs the event-driven capture: watermark + delta + consolidate-not-append

Three layers keep distill from duplicating moment-capture:

1. **Watermark.** `last-distill` records the event seq/timestamp of the previous
   pass. A distill considers only the event/outcome **delta since the watermark** —
   it never re-summarizes history.
2. **Different granularity.** Moment-capture writes *individual* durable facts as they
   surface; distill does the *retrospective synthesis* the moment couldn't see —
   cross-session patterns, recurring footguns, and the CONSOLIDATION (merge related
   entries) + PRUNE (drop dead facts). Its job is inherently de-duplicating.
3. **Update-over-append discipline** (already in the playbook). The distill output is
   a curation *diff* to the KB (merge / prune / add-a-missed-fact), never a blind
   re-summarize appended to a topic file. Even when it re-touches a known fact it
   updates in place.

So distill answers "over the last period, what durable thing did the fleet learn that
isn't yet captured, what's now stale, and what duplicates can merge?" — orthogonal to
moment-capture's "this specific fact, right now."

## D4 — Seed the discipline into the playbook (every HQ instance is born with it)

Split, like the charter work, into PROMPT and CODE:

**PROMPT (single-source seed, `hqInstructions`):**
- Add `distill` to the wake-class list (near `tick` / `self-check`), one clause: "a
  distillation pass is due — retrospectively distil the period's fleet activity into
  the KB and prune stale."
- Formalize the ritual (extend the Knowledge-base §): on a `distill` wake, read the
  event delta since the last distill, fold durable cross-cutting facts into the right
  topic file (update-over-append), PRUNE stale/dead entries + merge dupes; default
  SILENT, one line only on real curation; charter-level lessons still FLAG a seed/spec
  update (unchanged from the correction loop). Keep the NEVER-store-secrets rule.
- **Bump `hqPlaybookVersion` 6 → 7** (mandatory per CLAUDE.md: any `hqInstructions`
  change must bump it) so existing homes auto-upgrade on the next `gtmux hq`.

**CODE:**
- `internal/app/distill.go`: `shouldDistill(...)` (pure) + `distillSensor(now)` (marker
  I/O + `EmitControl`), called from `slowTickEval()` next to `selfCheckSensor`.
- `internal/hqfeed`: `ControlDistill = "gtmux:distill"`.
- `internal/hqwake`: a `distill` class (LLM-free signal line, like the others).
- `internal/app/hqfeedcmd.go`: render the new control record.

## Risks / open questions (for the commander)

- **Cadence dependency on %84's retention.** The event-volume floor N must be set with
  the final retention size in mind — reconcile once that work lands. Flag, don't block.
- **Heavy pass on the main HQ loop?** The distill is HQ reading a bounded event delta
  + editing KB files — bounded work, and #0 RESPONSIVENESS says push heavy work to a
  subagent. Open question: is a period distill light enough to run in-loop (like
  self-check) or should it be delegated? Recommend in-loop for the MVP (bounded delta),
  revisit if it proves heavy.
- **Scope of "fleet activity" to distill.** MVP = the event delta (`gtmux events`) +
  the attention ledger outcomes; NOT re-reading every transcript (too heavy, and the
  role boundary forbids HQ deep-reading repos itself — it would delegate that).

## Recommendation (one paragraph)

Add an **independent, LLM-free `distill` sensor** modeled exactly on `self-check`,
with a **weekly floor + event-volume floor + zero-change gate**; bound each pass to
the **event delta since a `last-distill` watermark** and make its KB output a
**consolidate/prune diff**, not an append, so it never duplicates moment-capture; and
**seed the ritual into the playbook** (new `distill` wake class + a formalized
Knowledge-base clause, `hqPlaybookVersion` 6→7) so every HQ instance carries the
discipline. Stay clear of events-retention and perception-self-heal (%84's lanes),
reconciling only the volume-floor number with the final retention size.
