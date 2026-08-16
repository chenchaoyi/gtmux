# hq-wake-protocol (delta)

## ADDED Requirements

### Requirement: Wake outcomes are journaled as audit records

Every wake batch's terminal outcome SHALL leave one audit record in the session
journal, so "what did gtmux tell HQ, and what did it fail to tell HQ" are journal
queries rather than reconstructions:

- a CONFIRMED delivery (the drain's ack, or the Enter-only repair confirming)
  SHALL append `gtmux:audit:wake-delivered` carrying the batch identifier and the
  full delivered payload — one record per coalesced batch, never per entry;
- a DROP SHALL append `gtmux:audit:wake-dropped` carrying the dropped line and its
  reason: `evicted` (queue overflow), `unconfirmed` (the ack budget exhausted,
  whether from the drain path or an abandoned Enter repair), or `superseded` (the
  delivery-side revalidation probe found the premise gone — the control trace the
  standing-knock rule already promised).

A requeue below the ack budget, and a revalidation that RE-RENDERS rather than
drops, SHALL write nothing: only terminal outcomes are journaled. Audit records
follow the session-events audit rules: trail, not debt.

The self-rotate sensor's fleet-movement counter SHALL exclude control records
(audit included): gtmux's own bookkeeping is never fleet movement, so a delivered
wake's own audit record can never re-arm the standing knock that produced it.

#### Scenario: A delivered batch is reconstructable from the journal

- **WHEN** a wake batch coalescing three queued lines is confirmed delivered
- **THEN** exactly one `gtmux:audit:wake-delivered` record carries the batch id and
  the full coalesced payload

#### Scenario: Every silent drop now leaves a trace

- **WHEN** a queued wake is evicted at the queue cap, dropped after its final
  unconfirmed attempt, or dropped because its premise died in the queue
- **THEN** a `gtmux:audit:wake-dropped` record names the reason and carries the
  dropped line

#### Scenario: A retry is not a drop

- **WHEN** an unconfirmed batch is returned to the queue below its ack budget
- **THEN** no wake-dropped record is written

#### Scenario: The trail does not feed the standing knock

- **WHEN** wake deliveries append audit records while a `self-rotate` breach stands
- **THEN** those records advance neither the fleet-movement counter nor the unread
  debt, so no knock re-arms on gtmux's own bookkeeping

## MODIFIED Requirements

### Requirement: Wake delivery is acknowledged, retried, and never silently dropped

A wake SHALL be removed from the delivery queue ONLY after its delivery is confirmed.
Delivery SHALL paste the line and submit it as separate steps (a paste buffer, then a
named Enter key). Confirmation SHALL be layered: when the HQ pane hosts a
hook-equipped agent, a prompt-submit event on that pane containing the batch's
identifier SHALL confirm the delivery deterministically (the driver receipt), with
the screen read retained as the fallback — reading the pane's capture (including
scrollback margin) for the batch's identifier. Any error from the paste or the
submit, and any unconfirmed read, SHALL return the batch to the queue for a later
attempt. A queue entry claimed by a drainer that never completed (a claim older
than 60 seconds) SHALL be reclaimed by the next drain.

A delivery whose paste landed but whose submit did not — the driver receipt reports
the payload unsubmitted, or the screen shows the batch identifier in the DRAFT
region — SHALL be handled as a distinct state: the next drain SHALL re-send ONLY the
Enter for that claim, after confirming the draft still holds that batch's text, and
SHALL NOT re-paste the payload or mint a new identifier. This bounds the
swallowed-Enter failure to one fast-drain interval instead of blocking the channel
behind its own stranded paste until the stale-queue degradation fires.

Every terminal outcome SHALL be journaled: a confirmed delivery as
`gtmux:audit:wake-delivered` (batch id + payload), and every drop — eviction, an
exhausted ack budget, a revalidation whose premise died — as
`gtmux:audit:wake-dropped` with its reason. "Never silently dropped" is thereby a
journal property, not only a queue property: the bounded, ANNOUNCED loss the ack
budget permits is announced in the stream, not merely by the degradation counter.

#### Scenario: A failed send keeps the nudge

- **WHEN** the paste or the Enter of a wake batch returns an error
- **THEN** the batch is returned to the queue and delivered by a later drain, and no
  queue entry is deleted

#### Scenario: An unconfirmed delivery is retried

- **WHEN** a wake batch is pasted and submitted but its identifier does not appear in the
  pane capture
- **THEN** the batch is returned to the queue and re-attempted on the next drain

#### Scenario: An entry that can never be confirmed does not loop forever

- **WHEN** a batch is pasted and submitted successfully but its delivery is never
  confirmed, drain after drain
- **THEN** it is re-sent at most 3 times in total (each carrying the same identifier)
  and then dropped, with the degradation raised and a `gtmux:audit:wake-dropped`
  record written — a send that ERRORS instead (nothing reached the pane) keeps
  retrying without limit, since it risks no duplicate

#### Scenario: A crashed drainer's batch is reclaimed

- **WHEN** a drainer claims a queue entry and dies before delivering it
- **THEN** a later drain reclaims the claim once it is older than 60 seconds and delivers
  the entry

#### Scenario: The HQ session's own submit event acks the wake

- **WHEN** a wake batch is pasted and submitted into a hook-equipped HQ pane and the
  pane's prompt-submit event carrying the batch identifier appears on the stream
- **THEN** the delivery is confirmed from the event — the confirmation does not
  depend on finding the identifier in a scrolled capture

#### Scenario: A swallowed Enter is repaired by Enter alone

- **WHEN** a wake batch's paste landed in the HQ composer but its Enter was swallowed
  (no submit event; the identifier sits in the draft region)
- **THEN** the next fast drain re-sends only Enter after confirming the draft still
  holds that batch, the identifier is unchanged, and the payload is never pasted a
  second time — the channel is not blocked until the stale-queue degradation

#### Scenario: A non-hook HQ falls back to the screen ack

- **WHEN** the HQ pane hosts an agent with no driver receipt capability
- **THEN** delivery confirmation uses the screen read exactly as before, unchanged

### Requirement: Unconsumed events wake HQ regardless of class

gtmux SHALL maintain a CONSUMPTION WATERMARK for HQ: the event sequence through which HQ
has read the stream. When the end of the stream exceeds that watermark continuously for the
aggregation window (`hqWake.unreadDebounceSec`, default 120 s), gtmux SHALL deliver an
`unread` wake naming the count of unconsumed events and the cursor to pull from, and SHALL
repeat it at `hqWake.unreadRepeatSec` (default 300 s) for as long as the watermark does not
advance. The wake SHALL carry NO severity or importance claim — classification is HQ's
judgment, made after the pull, because it requires context gtmux does not have.

The class SHALL queue at `PriorityStanding`, so it never drains ahead of a decision-dense
knock and is evicted first at the queue cap. Delivery SHALL be suppressed while an
undelivered knock is already queued (it adds nothing HQ will not already act on), without
clearing the standing debt.

Records authored by the HQ pane itself SHALL be excluded from the count: every delivered
wake re-enters the stream as HQ's own submission and its reply as HQ's own turn-end, so
counting them would make the sensor a perpetual source of its own input.

A pane-less lifecycle BLINK SHALL also be excluded from the count: a `SessionStart` with
no pane whose matching pane-less `SessionEnd` from the same agent arrives within a short
pairing window SHALL be excluded, and that end with it. A short-lived child process or
subagent whose hook fires without a pane is not something HQ can act on or attribute, so
it is not a read HQ owes.

gtmux's own AUDIT records (`gtmux:audit:*`) SHALL also be excluded from the count: they
document acts the supervision already performed — a wake it delivered, a send it made — so
counting them would make the trail the loop's fuel, one fresh unread record per delivered
knock. The exclusion is exactly the audit sub-namespace: `gtmux:` control records OUTSIDE
it (maintenance triggers, degradations, reconciles) carry new information and SHALL keep
counting.

The blink exclusion SHALL be defined by that PAIRING, never by the empty `pane` alone. An
empty pane is not a blink signature: it is also carried by every native (non-tmux) agent's
turns and by gtmux's own `gtmux:*` maintenance trigger records, and the `session` field
cannot discriminate because it holds the tmux session name and is empty on every pane-less
record by construction. Those two populations SHALL keep counting — decisively so, because
the class-wake channel fires only for a pane, making the `unread` knock the ONLY channel
they have. A pane-less `SessionStart` that is not quickly matched by an end SHALL keep
counting too: it is a native session coming online.

Excluded records SHALL remain in the stream: the exclusion is from the "owes HQ a read"
tally, never from the log. The supervisor's own delta pull SHALL show the SAME set the
count defines (see "Consumption is HQ's own explicit act"), and any other read SHALL return
them unchanged.

The `unread` line SHALL name the composition of its count — a compact by-source breakdown
alongside the total — so an echo-dominated or single-source accumulation is diagnosable
from the delivered line itself rather than requiring a manual stream read. The breakdown
SHALL be bounded so a wide fleet cannot grow the line past the delivery batch's budget.

The watermark SHALL NOT advance for any reason other than HQ consuming (below). In
particular, delivering a wake SHALL NOT advance it, and — mirroring the maintenance
sensors — the sensor SHALL be a complete no-op when no supervisor pane is resolvable,
since a watermark advanced with no HQ to tell would declare an HQ that arrived moments
later caught up on events it never saw.

On first use, with no watermark recorded, gtmux SHALL adopt the current end of the stream
rather than reporting the entire retained history as unconsumed.

#### Scenario: An unclassified event still knocks

- **WHEN** an event lands in the stream that no wake class claims, and HQ does not consume
  it within the aggregation window
- **THEN** an `» gtmux·unread  <n> unconsumed │ pull: gtmux events --since-seq <n> --json`
  line is delivered to the HQ pane

#### Scenario: The debt is not cleared by knocking

- **WHEN** an `unread` wake has been delivered and HQ has not consumed the stream
- **THEN** the watermark is unchanged and the knock repeats at the repeat interval

#### Scenario: A burst becomes one line

- **WHEN** twenty events land inside the aggregation window
- **THEN** one `unread` line is delivered, naming the total, rather than one line per event

#### Scenario: No supervisor means no watermark movement

- **WHEN** the sensor runs with no HQ pane resolvable
- **THEN** nothing is queued and the watermark is not advanced, so an HQ that appears later
  is still told about everything that accrued

#### Scenario: HQ's own screen traffic is not a backlog

- **WHEN** the only records past the watermark were authored by the HQ pane (a delivered
  wake echoed back as a submission, and HQ's own reply)
- **THEN** no `unread` wake is delivered

#### Scenario: A pane-less process blink is not a backlog

- **WHEN** the only records past the watermark are same-second `SessionStart`/`SessionEnd`
  pairs with an empty `pane` from a short-lived subprocess
- **THEN** no `unread` wake is delivered, and those records remain in the log — returned by
  `gtmux events --since-seq --all` and by any non-supervisor read

#### Scenario: gtmux's own audit trail is not a backlog

- **WHEN** the only records past the watermark are `gtmux:audit:*` records (delivered
  wakes, supervisor sends, a rotation)
- **THEN** no `unread` wake is delivered, the watermark steps over them exactly as it does
  for pure echo, and the records remain in the log

#### Scenario: A pane-less agent's work is still a backlog

- **WHEN** the records past the watermark are a pane-less agent's turn events, a `gtmux:*`
  maintenance trigger (not audit), or a pane-less `SessionStart` with no matching end in
  the pairing window
- **THEN** they count toward the debt and an `unread` wake is delivered — the blink
  exclusion keys on the Start/End pairing, never on the empty `pane` alone, and the audit
  exclusion keys on the `gtmux:audit:` sub-namespace alone

#### Scenario: The knock names what it counted

- **WHEN** an `unread` wake is delivered for three events from two sources
- **THEN** the line carries a compact breakdown of the count by source alongside the total,
  within the delivery batch's length budget

### Requirement: Consumption is HQ's own explicit act

The watermark SHALL advance only on an act by the supervisor, identified by the same
cwd-keyed role rule the radar uses (the invocation runs from the HQ home). Two acts SHALL
count:

- an UNFILTERED `gtmux events --since-seq <n>` delta read, advancing the watermark to the
  highest sequence the read covered — the pull HQ already performs on every wake, so the
  guarantee requires no new discipline of it;
- `gtmux events --ack <seq>`, the explicit writeback for a stream reconciled another way
  (e.g. a full `gtmux digest`), clamped to the end of the stream and monotonic.

A `--severity`-FILTERED read SHALL NOT advance the watermark, since it showed HQ a subset;
neither SHALL a read whose cursor starts AHEAD of the current watermark, since it skipped
the range between. Both remain permitted — they simply leave the debt standing. An
invocation from anywhere other than the HQ home SHALL NOT move the watermark.

The supervisor's own delta pull SHALL show the SAME set the count defines: an UNFILTERED
`--since-seq` read run from the HQ home SHALL omit the records the tally excludes — the
caller's own pane records, the pane-less blinks, and gtmux's own `gtmux:audit:*` trail —
and SHALL report on stderr how many it withheld, so the omission is never silent. `--all`
SHALL restore the raw view. Both forms SHALL advance the watermark: neither is a
`--severity` filter showing a SUBSET of what HQ owes; one shows exactly that set and the
other a superset of it. Any read that is not the supervisor's SHALL be returned unchanged.

This exists because the two sets disagreeing was, measured, the largest single cost in HQ
perception: 68.7 % of the records a knock's pull returned were HQ's own, so the median
knock spent an HQ turn reading its own trail to find one new fact.

The caller's own pane SHALL be identified from its environment (`$TMUX_PANE`), never by
resolving the HQ pane through tmux — the read path must stay free of tmux round-trips,
whose wedging has frozen a producer before.

A non-counting read SHALL fail loud when it plausibly IS the supervisor: an unfiltered
`--since-seq` read invoked from a cwd STRICTLY INSIDE the HQ home (a subdirectory such as
`notes/` or `knowledge/` — the measured `cd`-drift shape) SHALL emit a one-line stderr
warning that the read was not counted as consumption, naming the home to run from. The
read's output and exit code are unchanged. A read from an unrelated cwd SHALL stay
silent — a non-supervisor caller owns no watermark and is not nagged about one. This
mirrors `--ack`, which already refuses loudly outside the HQ home: two paths with one
meaning no longer have opposite failure modes.

#### Scenario: The everyday pull is the writeback

- **WHEN** HQ runs `gtmux events --since-seq <n> --json` from its home after a wake
- **THEN** the watermark advances to the end of what the read returned, and the `unread`
  knock stops

#### Scenario: A filtered read leaves the debt standing

- **WHEN** HQ reads `gtmux events --since-seq <n> --severity important`
- **THEN** the watermark does not move, and the unconsumed events knock again

#### Scenario: A skip-ahead read is not consumption

- **WHEN** HQ reads a delta whose cursor is higher than its own watermark
- **THEN** the watermark does not move, so the range jumped over is not silently dropped

#### Scenario: Only the supervisor can acknowledge

- **WHEN** `gtmux events --ack` is run outside the HQ home
- **THEN** it is refused and the watermark is unchanged

#### Scenario: A cd-drifted supervisor read warns instead of silently not counting

- **WHEN** an unfiltered `--since-seq` read runs from a subdirectory of the HQ home
  (e.g. `knowledge/`) so the watermark does not advance
- **THEN** a stderr line warns that this read was not counted as consumption and names
  the HQ home to run from, while stdout and the exit code are unchanged

#### Scenario: A bystander's read is not nagged

- **WHEN** an unfiltered `--since-seq` read runs from a cwd unrelated to the HQ home
- **THEN** no warning is emitted and the watermark does not move

#### Scenario: The pull shows the debt, not HQ's own trail

- **WHEN** HQ pulls a delta from its home whose range holds its own wake echo, its own
  reply, a pane-less blink pair, a `gtmux:audit:wake-delivered` record, and one turn-end
  from a worker pane
- **THEN** only the worker's turn-end is printed, stderr names how many records were
  withheld, and the watermark still advances to the end of the range

#### Scenario: --all restores the raw view and still consumes

- **WHEN** HQ pulls the same delta with `--all`
- **THEN** every record in the range is printed, nothing is reported as withheld, and the
  watermark advances

#### Scenario: A bystander's pull is not reshaped

- **WHEN** an unfiltered `--since-seq` read runs from a cwd unrelated to the HQ home
- **THEN** every record in the range is printed — the pull view is the supervisor's view of
  its own debt, and a caller with no debt has no echo to hide

### Requirement: A broken wake channel escalates out of band

Consecutive unconfirmed deliveries SHALL be counted, and reaching the failure limit
(3) SHALL raise a CRITICAL `wake-degraded` degradation exactly once per transition into
the degraded state: a control record at important severity appended to the session
JOURNAL (so the pull side actually carries it — a record written only to the feed spool
reaches no reader, the measured #647 failure shape), a best-effort HQ wake line, and a
desktop notification — the last because the alarm for a broken wake channel MUST NOT
depend on that channel. A confirmed delivery SHALL reset the counter, and recovery
SHALL NOT re-alert.

#### Scenario: The wake channel breaks

- **WHEN** three consecutive wake deliveries fail to confirm
- **THEN** a `wake-degraded` control record at important severity is appended to the
  journal — visible to `gtmux events` and counted toward the consumption debt — and a
  desktop notification is posted, once

#### Scenario: Recovery is silent

- **WHEN** a delivery confirms after a degradation was raised
- **THEN** the failure counter resets and no further degradation alert is emitted until
  the limit is reached again

### Requirement: HQ rotates itself through gtmux, not through raw tmux

`gtmux hq --rotate` SHALL resolve the live HQ pane itself and deliver that agent's own
conversation-reset input to it, and SHALL restart the session-health window so the knock
does not repeat about the session it just retired. It SHALL report a clear failure when no
HQ pane is resolvable.

This exists so the rotation stays inside HQ's role boundary: HQ decides and hands off, gtmux
performs the tmux mechanics, and the HARD role whitelist (HQ runs no concrete command, and
never sends navigation keys into a TUI) is not weakened to make self-rotation possible.

Rotation SHALL leave a durable chain in the journal. The `--rotate` act SHALL append
`gtmux:audit:rotate` naming the retiring session id and the reset input typed; when the
health sensor later observes the HQ session id REPLACE a known one (the rotation
settling — or any session change, including a hand-typed reset), it SHALL append
`gtmux:audit:hq-session` naming the successor and the predecessor before the old window
is discarded. A FIRST sighting appends nothing: it replaces no one, nothing is being
lost (the live resume record still names that session), and a healthy first sight
keeping the stream untouched is the sensors' silence discipline. The state files keep
only the current session (unchanged); the journal is where the predecessor chain
survives, so "which session preceded this HQ, and where is its transcript" is a journal
query instead of information destroyed at every handoff.

#### Scenario: Rotation is a gtmux verb

- **WHEN** HQ has brought its board and knowledge base current and run `gtmux hq --rotate`
- **THEN** gtmux delivers the reset input into the HQ pane, appends a
  `gtmux:audit:rotate` record naming the retiring session, and the health window restarts

#### Scenario: Rotation without a supervisor fails loudly

- **WHEN** `gtmux hq --rotate` runs with no live HQ pane
- **THEN** it exits non-zero with a message naming the missing supervisor, rather than
  silently succeeding

#### Scenario: The successor chain survives the handoff

- **WHEN** the health sensor first observes the successor session id after a rotation
- **THEN** a `gtmux:audit:hq-session` record names both ids before the predecessor's
  window is discarded, while a FIRST-ever sighting and an unchanged session id on a
  later tick each append nothing
