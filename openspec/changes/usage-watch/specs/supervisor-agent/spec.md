## MODIFIED Requirements

### Requirement: Waiting-event nudge into the supervisor

The system SHALL, when a tmux agent enters waiting and a supervisor session is
live, inject ONE compact line — the location, waiting kind, and title — into the
supervisor's pane (send-keys + Enter), riding the notification pipeline's
existing dedup so an unchanged waiting state is not re-nudged. The SAME channel
SHALL carry usage warnings (`[gtmux] usage·warn <loc> — <detail>`, deduped per
session+layer — see `usage-watch`). It SHALL never nudge the supervisor about
its own waiting states, SHALL be a no-op when no supervisor session is live, and
SHALL be disableable via configuration (`hqNudge: false`, default on).

#### Scenario: Agent blocks, supervisor learns

- **WHEN** another agent enters waiting (permission/plan/question) while an hq
  session is live
- **THEN** one `[gtmux] waiting·<kind> <loc> — <title>` line is typed into the
  hq pane, at most once per waiting transition

#### Scenario: Usage warning reaches the supervisor

- **WHEN** a session breaches (or projects into) a usage layer while HQ is live
- **THEN** one `[gtmux] usage·warn <loc> — <detail>` line is typed into the hq
  pane, at most once per session+layer

#### Scenario: Never about itself, off when absent or disabled

- **WHEN** the supervisor itself is the waiting pane, or no hq session is live,
  or `hqNudge` is false
- **THEN** nothing is injected
