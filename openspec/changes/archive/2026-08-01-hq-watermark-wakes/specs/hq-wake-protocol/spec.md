# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Single wake channel with deterministic classes

All gtmux-initiated injections into the HQ pane SHALL flow through one wake
channel with exactly two classes: IMMEDIATE wakes for decision-dense events —
`waiting·<kind>`, `resolved` (a wait cleared), `asks`, `done` (unattended
completion), `crash` (a turn that died on an agent/API failure), `goal-changed`
(a user-direct prompt in a non-HQ pane), `new-session` (a newly sensed agent
session), `reap-suggest`, `feed-degraded`, and the standing resource/limits
warnings — and a periodic `tick` wake. The standing set SHALL additionally include the
periodic MAINTENANCE classes `distill` and `self-check`, raised by the serve slow-tick's
own sensors, and the completeness class `unread`. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). Producer-heartbeat receipt suppression
(`feedSupersedesReceipts`) is REMOVED: the wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).

The class list SHALL be understood as a PRIORITY vocabulary, not a coverage guarantee: a
class states what HQ should look at first, and NOT the set of events HQ can learn about.
Completeness is guaranteed separately, by the consumption watermark below. An event that
matches no class SHALL therefore still reach HQ.

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while the hq-feed daemon
  heartbeat is fresh
- **THEN** the wake line is still typed into the HQ pane (the daemon's health no
  longer suppresses the knock)

#### Scenario: An event matching no class still reaches HQ

- **WHEN** a session that gtmux never dispatched to reaches a turn-end that asks no
  question — matching neither `done` (no ledger entry) nor `asks`
- **THEN** no immediate class fires, and the event is nonetheless delivered to HQ by the
  unconsumed-events knock rather than being silently dropped

## ADDED Requirements

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
