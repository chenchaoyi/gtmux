# supervisor-agent — delta

## MODIFIED Requirements

### Requirement: Waiting-event nudge into the supervisor

The system SHALL, when a tmux agent enters waiting and a supervisor session is live,
wake the supervisor with ONE compact line carrying the location, waiting kind, and title,
riding the notification pipeline's existing dedup so an unchanged waiting state is not
re-nudged. The SAME channel SHALL carry usage warnings (`usage·warn`, deduped per
session+layer — see `usage-watch`) and the lifecycle watchdog's escalation
(`stuck·waiting`). It SHALL never wake the supervisor about its own waiting states, SHALL
be a no-op when no supervisor session is live, and SHALL be disableable via configuration
(`hqNudge: false`, default on).

Delivery SHALL go through the single wake channel (see `hq-wake-protocol`): the declared
class, the `» gtmux·<class>` signal format, and the channel's draft guard, ack, and queue.
No caller SHALL type into the supervisor's pane directly, and no requirement SHALL
describe a delivery mechanism of its own — a superseded requirement that is merely
contradicted by a newer one, rather than retired, keeps blessing the code that still obeys
it.

#### Scenario: Agent blocks, supervisor learns

- **WHEN** another agent enters waiting (permission/plan/question) while an hq
  session is live
- **THEN** one `waiting·<kind>` wake line reaches the hq pane, at most once per waiting
  transition

#### Scenario: Usage warning reaches the supervisor

- **WHEN** a session breaches (or projects into) a usage layer while HQ is live
- **THEN** one `usage·warn` wake line reaches the hq pane, at most once per session+layer

#### Scenario: Every injection is draft-guarded

- **WHEN** any of these wakes fires while the user is composing in the hq pane
- **THEN** nothing is typed: the wake queues and lands once the box is empty

#### Scenario: Never about itself, off when absent or disabled

- **WHEN** the supervisor itself is the waiting pane, or no hq session is live,
  or `hqNudge` is false
- **THEN** nothing is injected
