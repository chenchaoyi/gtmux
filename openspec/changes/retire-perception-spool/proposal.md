# retire-perception-spool — one event stream, read where it is consumed

Origin: the commander's requirement that HQ↔pane traffic be ABSOLUTELY reliable.
Auditing the perception path found a copy layer whose only reader the playbook
itself tells HQ not to use — and every part of it is either redundant with the
journal's own guarantees or a measured source of the exact loss it was built to
prevent.

## Why — the measured findings

1. **The spool's only reader is off the recommended path.** `gtmux hq-feed
   --tail` is the sole consumer of the spool, and the seeded playbook says "You
   do NOT need to tail it — wake lines knock, and you pull deltas". HQ's real
   perception is wake-knock + `gtmux events --since-seq` + the watermark's
   completeness net. The daemon copies every journal event for a subscriber
   that, per its own charter, does not exist.
2. **Two streams mean two truths — and that produced real loss.** #647: control
   records written only to the spool reached NO reader; 75 degradation records
   accumulated invisible to every query. The class of bug "the copy and the
   journal disagree" exists only because there is a copy.
3. **The replay→follow handover loses events by design.** `FollowSpool` (and
   `events.Follow`) replay a time window, then open the live file and seek to
   END — an event appended between the replay's last read and that seek is
   emitted by neither. The pull path has no such window: `--since-seq` is
   seq-exact.
4. **A watchdog guards a process nothing depends on.** The serve slow-tick
   carries a heartbeat monitor, exponential-backoff restart gate, and a
   two-failure CRITICAL escalation — all supervising the copyist. The
   zero-loss guarantees live elsewhere (journal seq + watermark ack + unread
   re-knock) and keep working with the daemon gone.
5. **Gap detection runs at the wrong moment.** The daemon checks for a cursor
   gap once, at ITS startup — not when HQ actually reads. A gap that opens
   while the daemon is healthy is announced by nobody.

## What changes

- **The spool, the daemon, and the feed watchdog are deleted.** `gtmux hq-feed`
  (--daemon/--tail/--status) retires; `internal/hqfeed` keeps only what other
  subsystems genuinely use — the surfacing tiers/threshold (quiet) and the
  journal control-record names. The slow-tick loses feedWatchdog, the restart
  gate, and its state files.
- **Gap detection moves to the consumption point.** `gtmux events --since-seq
  <n>` now uses the journal's `ReadSince` and, when a sequence hole is
  detected, prints a CRITICAL warning line telling HQ to rebuild from a
  `gtmux digest` snapshot before acking — the reader is told at the exact
  moment the gap matters, every time it reads, instead of once at a daemon's
  startup.
- **The `feed-degraded` wake class retires** (its raiser was the watchdog);
  `wake-degraded` stays — the wake side still has a real failure mode and a
  real watchdog. The `reconcile` control record retires with the daemon whose
  restarts it announced; HQ's own restarts already rebuild via the startup
  briefing's digest snapshot.
- **The playbook drops the spool-daemon teaching** (perception = journal +
  knock + pull; a gap warns you at read time), v28 → v29. The retired
  vocabulary (`hq-feed`, `feed-degraded`) joins the check-design retired list
  and the playbook ban-list test, so it cannot quietly return.

## Non-goals

The capture queue (`pending-distill spool` in capturecmd.go) is an unrelated
namesake — untouched. The wake channel, watermark/ack, and unread completeness
net are the load-bearing reliability mechanisms — untouched by design; this
change removes a layer BESIDE them, not under them. No new push channel: HQ's
perception stays pull-on-wake.

## Impact / risk

Deletion-heavy (daemon, tail, watchdog, spool I/O, their tests); the one new
behavior is the gap warning on `--since-seq` reads, tested both ways. The
failure mode this change can introduce — "HQ misses events with no spool as
backup" — is exactly what the watermark already guards: events are retained in
the journal (8 MB × 2 generations), the unread debt re-knocks until HQ acks,
and a retention overrun now WARNS AT READ TIME, which the spool never did.
Old `~/.local/share/gtmux/hq-feed/` state files are left on disk (inert,
harmless); no migration.
