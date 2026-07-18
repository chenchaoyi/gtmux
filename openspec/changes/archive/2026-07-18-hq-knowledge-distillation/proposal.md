# Change: hq-knowledge-distillation

> STATUS: investigation / proposal only — awaiting commander (HQ) approval before
> implementation. No code is written by this change yet.

## Why

The HQ playbook already makes the knowledge base "YOUR SINGLE MOST IMPORTANT JOB"
and tells HQ to **Capture** durable facts as they surface and to **Iterate:
periodically review — correct what's stale, prune what's dead, merge duplicates."**
But *Capture* is event-driven (it fires the moment something is learned) while
*Iterate* is prose-only aspiration — **nothing triggers it.** With no clock
discipline that survives a `/compact` or context reset, the periodic
consolidation-and-prune never reliably happens, so the KB accretes moment-captured
entries and slowly rots: duplicates pile up, stale facts linger, and cross-session
patterns that no single in-the-moment capture could see are never distilled.

This change gives HQ a **scheduled self-summary → knowledge-distillation** ritual:
on a cadence, HQ retrospectively distills what happened across the whole fleet since
the last pass into `knowledge/*.md` and prunes what's dead — the same LLM-free
sensor → control-record → HQ-acts pattern that already backs `self-check`, but with
its own trigger, cadence, and purpose.

## What Changes

- **A periodic distill trigger (new).** A deterministic, LLM-free sensor in the serve
  slow-tick raises a new `distill` control record when a distillation pass is due
  (cadence below). gtmux never runs an LLM in the timing loop; HQ does the actual
  curation — exactly the `self-check` split.
- **A new `distill` wake class** taught in the playbook's wake-class list, distinct
  from `self-check` (own-artifact HEALTH housekeeping) and `tick` (the user-facing
  summary brief).
- **A watermark** (`last-distill`, a seq/timestamp marker in state) so each pass
  distills only the event/outcome DELTA since the previous one — never re-summarizing
  history, never duplicating what moment-capture already wrote.
- **A formalized distillation ritual in the seeded playbook** (`hqInstructions`),
  turning "periodically review" into a triggered, bounded discipline: on a `distill`
  wake, read the fleet's event delta since the watermark, fold durable cross-cutting
  facts into the right topic file (update-over-append), PRUNE stale/dead entries and
  merge duplicates; default SILENT, one line only on real curation; charter-level
  lessons still flag a seed/spec update. **`hqPlaybookVersion` bumps 6 → 7** so every
  existing HQ home auto-upgrades on the next `gtmux hq`.

## Capabilities

### Modified Capabilities

- `supervisor-agent` — adds the periodic knowledge-distillation trigger, the
  watermark-bounded delta discipline, and the seeded-playbook ritual. ALSO folds in the
  **perception self-heal charter discipline** (on `feed-degraded`/`wake-degraded`:
  verify by pull before nagging, stay silent when perception is fresh, restart only via
  a dispatched worker) so the seeded playbook version bumps ONCE — the code-side disk /
  feed-daemon hardening ships as a SEPARATE, code-only change that touches no playbook.

## Non-goals

- **Not events-log retention, and not perception self-heal** — those two are already
  dispatched to another worker (`perception-self-heal-hardening` + events retention).
  This change only READS events (within whatever retention window that work sets) to
  distill; it does not change rotation, feed-degraded, or wake-degraded handling. The
  one dependency to reconcile with that work: the distill cadence must fire before a
  busy fleet's events rotate out (see design — an event-volume floor, since retention
  is size-based).
- Not a change to `self-check` (health housekeeping) or `tick` (the summary brief) —
  distill is a third, independent ritual that reuses only their sensor *infrastructure*.
- No new user-facing CLI surface required for the MVP (the trigger is internal); a
  `gtmux`-side manual "distill now" is a possible later add, out of scope here.
