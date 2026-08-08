# hq-wake-protocol (delta)

## MODIFIED Requirements

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

The exclusion SHALL be defined by that PAIRING, never by the empty `pane` alone. An empty
pane is not a blink signature: it is also carried by every native (non-tmux) agent's turns
and by gtmux's own `gtmux:*` maintenance trigger records, and the `session` field cannot
discriminate because it holds the tmux session name and is empty on every pane-less record
by construction. Those two populations SHALL keep counting — decisively so, because the
class-wake channel fires only for a pane, making the `unread` knock the ONLY channel they
have. A pane-less `SessionStart` that is not quickly matched by an end SHALL keep counting
too: it is a native session coming online.

Excluded records SHALL remain in the stream: the exclusion is from the "owes HQ a read"
tally, never from the log. The supervisor's own delta pull SHALL show the SAME set the
count defines (see "Consumption is HQ's own explicit act"), and any other read SHALL return
them unchanged.

The `unread` line SHALL name the composition of its count — a compact by-pane/kind
breakdown alongside the total — so an echo-dominated or self-feeding accumulation is
diagnosable from the delivered line itself rather than requiring a manual stream read.

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

#### Scenario: A pane-less agent's work is still a backlog

- **WHEN** the records past the watermark are a pane-less agent's turn events, a `gtmux:*`
  maintenance trigger, or a pane-less `SessionStart` with no matching end in the pairing
  window
- **THEN** they count toward the debt and an `unread` wake is delivered — the blink
  exclusion keys on the Start/End pairing, never on the empty `pane` alone

#### Scenario: The knock names what it counted

- **WHEN** an `unread` wake is delivered for three events from two sources
- **THEN** the line carries a compact breakdown of the count by pane/kind alongside the
  total, within the wake-line length cap

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
caller's own pane records and the pane-less blinks — and SHALL report on stderr how many it
withheld, so the omission is never silent. `--all` SHALL restore the raw view. Both forms
SHALL advance the watermark: neither is a `--severity` filter showing a SUBSET of what HQ
owes; one shows exactly that set and the other a superset of it. Any read that is not the
supervisor's SHALL be returned unchanged.

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
  reply, a pane-less blink pair, and one turn-end from a worker pane
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
