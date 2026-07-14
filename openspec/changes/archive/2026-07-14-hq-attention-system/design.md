## Context

gtmux already has most of the raw materials this system needs:

- **A durable, rotated event journal.** `internal/events` appends one JSON record per
  lifecycle event to `events.jsonl`, rotates at a size cap (default 20 MB → one
  `events.1.jsonl` generation, ~40 MB ceiling), and each record already carries a
  deterministic `severity` (`routine|notable|important`, stamped at the single append
  path) plus `summary`/`class`. `gtmux events --follow` tails it rotation-aware.
- **A no-LLM periodic host.** `gtmux serve`'s hub runs `slowTickEval` every ~20 s from a
  single goroutine (no nudge race). It already runs `watchdogSweep` (escalate a stuck
  `waiting` pane) and the resource/limits evaluators. This is the natural home for a
  mechanical feed-watchdog.
- **A supervisor with durable state.** `gtmux hq` seeds an idempotent playbook
  (`AGENTS.md`) + a situation board (`notes/board.md`) + a knowledge base, all persisting
  across context resets. HQ is taught to read `digest`/`events`/`tasks` and drive via
  `send`/`spawn`.
- **A dispatch ledger.** `internal/dispatch` stores one JSON file per task under
  `~/.local/share/gtmux/tasks/`; `gtmux tasks` joins it with the live radar. Status is
  *derived* from the pane, not stored.
- **The nudge pipe we are splitting.** `internal/hqnudge` + `nudgeSupervisor`/
  `nudgeResolved` type `[gtmux] …` lines VISIBLY into the HQ pane — the coupling this
  change breaks.

What is missing for robust, split perception: a monotonic sequence + consumed cursor
(zero-loss resume), a heartbeat + mechanical watchdog, a degradation alert, and a
silent (non-pane-visible) way to feed HQ the full stream.

## Goals / Non-Goals

**Goals:**
- Feed HQ the FULL event stream silently and crash-safely; surface to the USER only
  what HQ judges high-value.
- Zero event loss across a feed crash/restart (cursor-based catch-up), gap-detectable.
- A perception OUTAGE is known in seconds (degradation ⇒ CRITICAL), self-healing
  mechanically first (no-LLM), escalating only after self-heal fails.
- All persistence stays **rollable**: journal rotates, spool rotates, ledger archives.

**Non-Goals:**
- No new phone/notification channel — CRITICAL rides the existing push pipeline.
- No LLM in the health/timing loop.
- No change to the journal's rotation model or the `agents --json` / SSE contracts.
- Not solving "HQ truly internalized every line" — the LLM absorption guarantee is the
  board + startup reconciliation's job, not the byte-level feed's.

## Decisions

### D1 — Silent feed = gtmux daemon → spool file → HQ tails the spool

The core tension: an LLM agent in tmux only reacts to (a) text typed into its pane
(VISIBLE — the thing we're removing) or (b) a harness **background task** emitting output
(silent — enters HQ's context as tool output, not a user-visible print). A silent feed
must use (b). But a harness background task is owned by HQ's harness — **gtmux cannot
mechanically restart it**, which the commander's robustness bar requires.

Resolution: **decouple with a spool file.**
- A gtmux-managed daemon **`gtmux hq-feed`** (no LLM, pidfile'd) tails `events.jsonl`
  from a persisted cursor and appends each event to a **spool** the daemon owns
  (`~/.local/share/gtmux/hq-feed/spool.jsonl`, itself rotated). It writes a heartbeat
  every 30 s.
- HQ's silent consumer is a trivial harness background task — effectively
  `tail -f hq-feed/spool.jsonl` (wrapped as `gtmux hq-feed --tail` so it's cursor-aware
  and rotation-aware). Its output wakes HQ; HQ reads, judges, prints only high-value.
- The **gtmux-side watchdog** (in `serve`'s slow-tick) supervises the DAEMON — a real
  gtmux process it CAN restart via pidfile. The daemon does all the fragile work
  (journal tail, cursor, gap detection, heartbeat); HQ's tail is dumb and, if it dies,
  the harness re-wakes HQ (feed-task-exit) to restart it while the spool keeps the data.

*Alternative considered:* HQ runs `gtmux events --follow` directly as its background
task (no daemon). Simpler, but then the only "watchdog" able to restart it is HQ's own
LLM — violating the no-LLM-watchdog requirement and giving no mechanical self-heal. The
spool indirection is the price of a mechanical, silent, restartable feed.

### D2 — Monotonic sequence via a flock'd counter (cursor + gap detection)

Add `Seq int64` to `events.Record`, assigned at `Append` time from a counter file
`events.seq`, guarded by an advisory `syscall.Flock` (cgo-free on darwin/linux) so
concurrent hooks get distinct, increasing seqs. The cursor is simply "last seq consumed";
resume = read the journal (across both generations) and emit records with `seq > cursor`,
sorted by seq. A hole in consumed seqs = a gap ⇒ trigger reconciliation.

*Alternative considered:* byte-offset cursor. Rejected — rotation invalidates offsets and
it gives no gap signal. *Alternative:* `ts`-only ordering. Rejected — ties within a second
and no gap detection. The flock adds microseconds to an already best-effort, low-frequency
append; a crash mid-critical-section can leave at most a one-seq phantom gap, which only
causes a harmless extra reconciliation (idempotent).

### D3 — Heartbeat + watchdog thresholds (the commander's fixed N)

Per §6.4 of the design, fixed: **heartbeat 30 s · stale 90 s (3 missed beats) · self-heal
2 failures before escalation · HQ polling backstop 5 min.** The watchdog, on each ~20 s
slow-tick:
1. If HQ is not live (`findHQPane() == ""`) → do nothing (no feed needed, no cost).
2. Else ensure the daemon is healthy: pidfile alive AND heartbeat age ≤ 90 s.
3. If unhealthy → restart the daemon (fork `gtmux hq-feed --daemon`), count the attempt.
4. Two consecutive failed restarts → **degradation ⇒ CRITICAL**: write a synthetic
   `feed-degraded` control record to the spool AND fire one visible `hqnudge` to the HQ
   pane ("⚠ perception feed down — on polling backstop"). This is the ONE place the
   watchdog is allowed to be visible, because a perception outage is exactly what must
   not stay silent.
5. A recovered feed clears the degraded marker (dedup by-tier, like the existing
   `markerChanged` nudges) so recovery doesn't re-alert.

*Serve-off fallback:* the mechanical watchdog needs `gtmux serve` running (typical on a
machine using the phone app / a live HQ). When serve is off, HQ's harness restarts the
tail on feed-task-exit, and HQ's own 5 min `digest` poll backstops state. Documented, not
hidden — a degraded-but-not-lost posture.

### D4 — Severity tiers map to surfacing tiers (no new classifier)

The design's CRITICAL/NORMAL/QUIET are a *surfacing* concern; the event `severity`
(`routine|notable|important`) is the *event* concern and already exists. Map, don't
duplicate:
- `important` (Waiting, asking Stop) → **CRITICAL** surfacing.
- `notable` (report Stop, dispatch done/stuck, lifecycle) → **NORMAL** surfacing.
- `routine` (submit, resolved, send-landed, working tick) → **QUIET** (ledger only).
- Two runtime overlays the static tier can't express stay in the ledger/HQ layer:
  **late-promotion** (a QUIET item promotes when related events accrue past a threshold)
  and **degraded-feed** (always CRITICAL regardless of the triggering event's tier).
The `feed-degraded` control record is stamped `important` at the source.

### D5 — Attention ledger = extend `gtmux tasks`, additively

Grow the dispatch `Task` with additive, optional fields so old entries still load:
`Tier`, `Priority` (re-orderable int), `Surfaced` (bool + when), `Disposition` (free
text: auto-answered / relayed / todo), `FirstSeen`, `LastUpdate`, `Archived`/`ArchivedAt`.
`gtmux tasks` gains `--verbose` (show archived + surfaced/disposition columns) and a
re-order/promote path HQ can drive. Archival keeps the ledger rollable: closed items move
under `tasks/archive/` (or a compacted `archive.jsonl`) so the live set stays small.

*Alternative considered:* a brand-new ledger store. Rejected — `gtmux tasks` is already
the needs-you ledger the playbook and phone use; a parallel store would fork the truth.

### D6 — Surfacing config: `gtmux quiet` + `surfaceTier` in config.json

Reuse the existing `config.json` merge (`setConfigKey`). Add `surfaceTier`
(`critical|normal|quiet`, default `normal` = surface NORMAL and up) and a `quiet` boolean
(a fast toggle equivalent to `surfaceTier=critical`). `gtmux quiet [on|off|status]` is the
ergonomic front door. gtmux exposes the resolved threshold; HQ reads it and gates its
prints accordingly (the gate lives in HQ's judgment, config just sets the bar).

### D7 — Self-check triggers sensed by gtmux (no-LLM), executed by HQ

Split hard/soft per §5. gtmux (slow-tick, no LLM) SENSES and raises a **self-check
trigger** when: idle ≥ ~2 h with no CRITICAL/NORMAL surfaced AND ≥ ~12 h since last
self-check (the "user is resting" case); OR a threshold trips (open ledger items > ~200,
journal over its rotation ceiling, cursor gap); OR the daily floor (≥ 24 h since last).
Rate-limited to ≤ 1/h. The trigger is delivered like any feed control record (a
`self-check` line into the spool, marked so it does not itself count as user-facing).
HQ then performs the review/cleanup on ITS OWN artifacts (log health, ledger archival,
memory-quality pass) — all within its existing write-own-notes whitelist — and prints a
one-line brief ONLY when it did real work; a severe finding (rotation broken, gap,
memory mass-invalid) escalates CRITICAL.

## Risks / Trade-offs

- **[Spool indirection adds a moving part]** → It is a plain rotated append file; the
  daemon is small and pidfile-guarded, and the watchdog + HQ-harness both restart it. The
  spool decouples the fragile tail from HQ's consumer so neither failure loses data.
- **[Watchdog depends on `gtmux serve`]** → Documented fallback (HQ harness + 5 min poll).
  Most target machines run serve as a launchd service already. A follow-up could move the
  watchdog into the daemon's own self-supervising parent if serve-independence is needed.
- **[flock counter contention]** → Appends are low-frequency and best-effort; a phantom
  gap from a mid-critical-section crash only triggers an idempotent reconciliation.
- **[Silent feed = HQ might under-surface]** → Config threshold + `--verbose` retro-query
  let the user pull anything back; late-promotion catches accreting low-value items;
  degraded-feed can never be silenced. The default `surfaceTier=normal` matches today's
  attention level, so the change is not a behavior regression, only a de-flood.
- **[HQ-behavior lives in a seed the live home won't auto-update]** → As with prior HQ
  changes, seed edits affect FRESH homes; existing homes need a manual re-seed. Called out
  in tasks + memory so the commander's live HQ is updated deliberately.

## Migration Plan

Ship in the proposal's phase order, each its own PR, each independently valuable:
①  perception feed (seq + cursor + daemon + heartbeat + watchdog + degradation +
   reconciliation) — the foundation, land it solid first.
②  severity→surfacing map + attention-ledger extension (fields, archive, `--verbose`,
   late-promotion).
③  `gtmux quiet` + `surfaceTier` config + HQ print-gating seed update.
④  self-check triggers + HQ self-maintenance seed update.
Rollback is per-phase: the daemon/quiet/ledger fields are additive; reverting a phase
leaves the journal + existing nudges intact.

## Open Questions

- Should the daemon spool carry the FULL stream or pre-filter to `notable`+ to keep HQ's
  context lean? (Leaning full — the design says HQ is omniscient; HQ filters. Revisit if
  context cost bites.)
- Ledger archive format: per-file under `tasks/archive/` (consistent with today) vs. a
  single compacted `archive.jsonl` (fewer inodes). (Leaning per-file for phase ②; compact
  later if needed.)
- Is serve-dependence for the watchdog acceptable for v1, or should the daemon
  self-supervise from the start? (Leaning serve for ①; note as a hardening follow-up.)
