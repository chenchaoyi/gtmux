# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Single wake channel with deterministic classes

All gtmux-initiated injections into the HQ pane SHALL flow through one wake
channel with exactly two classes: IMMEDIATE wakes for decision-dense events —
`waiting·<kind>`, `resolved` (a wait cleared), `asks`, `done` (unattended
completion), `crash` (a turn that died on an agent/API failure), `goal-changed`
(a user-direct prompt in a non-HQ pane), `new-session` (a newly sensed agent
session), `reap-suggest`, `wake-degraded`, and the standing resource/limits
warnings — and a periodic `tick` wake. The standing set SHALL additionally include the
periodic MAINTENANCE classes `distill` and `self-check`, the SESSION-HEALTH class
`self-rotate`, raised by the serve slow-tick's own sensors, and the completeness class
`unread`. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). The wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).
A retention gap is not a wake class: the pull itself announces it (the read-time
gap warning), because the reader is present at the exact moment it matters.

The class list SHALL be understood as a PRIORITY vocabulary, not a coverage guarantee: a
class states what HQ should look at first, and NOT the set of events HQ can learn about.
Completeness is guaranteed separately, by the consumption watermark below. An event that
matches no class SHALL therefore still reach HQ.

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while every other
  perception mechanism is healthy
- **THEN** the wake line is still typed into the HQ pane — no mechanism's
  health suppresses the knock

#### Scenario: An event matching no class still reaches HQ

- **WHEN** a session that gtmux never dispatched to reaches a turn-end that asks no
  question — matching neither `done` (no ledger entry) nor `asks`
- **THEN** no immediate class fires, and the event is nonetheless delivered to HQ by the
  unconsumed-events knock rather than being silently dropped

#### Scenario: The supervisor's own session is a wakeable subject

- **WHEN** the HQ session itself crosses a session-health threshold
- **THEN** a `self-rotate` wake is delivered on the same channel as every other class — the
  supervisor is not exempt from being the subject of a knock

### Requirement: Wake queue is prioritized and bounded

Queue entries SHALL carry the priority of their wake class: decision-dense classes
(`waiting`, `asks`, `goal-changed`, `crash`, `wake-degraded`) outrank
outcome classes (`done`, `resolved`, `new-session`, `reap-suggest`, `tick`), which
outrank standing warnings (`resource·warn`, `limits·warn`). A drain SHALL emit entries
highest-priority first and oldest-first within a priority, SHALL bound one coalesced
delivery by BOTH a line count (8) and a payload size (~800 chars — large enough to be
useful, small enough that an agent TUI renders it rather than folding it into a
paste placeholder the ack could not read), and SHALL indicate that entries were held
back. The queue SHALL
be bounded (200 entries), evicting the lowest-priority oldest entry when full — a
standing warning that will re-fire is preferred over a decision-dense wake that will not.
Entries written by an earlier version SHALL remain drainable.

#### Scenario: A decision-dense wake overtakes standing warnings

- **WHEN** a `goal-changed` wake is queued behind several `resource·warn` entries
- **THEN** the next drain delivers the `goal-changed` line first

#### Scenario: A backlog cannot become one unbounded paste

- **WHEN** more than 8 entries are due in one drain
- **THEN** at most 8 lines are coalesced into the delivery, it indicates the remainder is
  queued, and the rest are delivered by the following drain
