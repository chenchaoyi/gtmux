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
own sensors. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). Producer-heartbeat receipt suppression
(`feedSupersedesReceipts`) is REMOVED: the wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while the hq-feed daemon
  heartbeat is fresh
- **THEN** the wake line is still typed into the HQ pane (the daemon's health no
  longer suppresses the knock)

## ADDED Requirements

### Requirement: Periodic maintenance wakes at standing priority

The periodic maintenance triggers `distill` and `self-check` SHALL be delivered as wake
lines on the same draft-guarded, acknowledged channel as every other class, at
`PriorityStanding` — the lowest priority. They SHALL therefore never be drained ahead of a
decision-dense knock, and SHALL be the first evicted when the wake queue is at its cap,
because a standing condition re-fires on its own cadence while a decision knock does not.
A maintenance trigger SHALL NOT be delivered silently to the feed alone: an event-driven
supervisor with no timer of its own cannot self-initiate a periodic ritual, so a
maintenance record with no knock is a trigger delivered to nobody.

#### Scenario: Maintenance never preempts a blocked agent

- **WHEN** a `distill` wake and a `waiting` wake are queued together
- **THEN** the `waiting` line drains first — the maintenance line waits its turn

#### Scenario: Maintenance is evicted before a decision knock

- **WHEN** the wake queue is at its cap and a new decision-dense wake arrives
- **THEN** a queued `distill` / `self-check` line is evicted rather than the decision wake,
  since the maintenance trigger re-fires on its next cadence boundary

#### Scenario: A due maintenance pass actually reaches the pane

- **WHEN** a maintenance sensor decides a pass is due and an HQ pane is live
- **THEN** a `» gtmux·distill …` / `» gtmux·self-check …` line is delivered to that pane,
  not only written to the perception feed
