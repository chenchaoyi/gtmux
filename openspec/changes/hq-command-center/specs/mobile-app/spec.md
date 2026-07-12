## ADDED Requirements

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL
present a dedicated HQ command center — NOT the generic Chat/Terminal detail. It
SHALL contain: a status strip (fleet counts + subscription-window %), a fleet
board listing every agent from `/api/digest` (needs-you first; each row shows
state, location, agent, goal, and — when waiting — its ask), and a command
console (a conversation with the supervisor plus a command bar with free text
and quick-command chips). Commands are HQ-mediated: the command bar addresses the
supervisor, which drives the fleet; the HQ screen has NO
direct-send input; direct control lives in each worker's own detail, reached by
long-pressing a fleet row.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the fleet board + command console,
  not the generic Chat/Terminal segmented detail

#### Scenario: Sense the fleet and command through HQ

- **WHEN** the user is in the HQ command center
- **THEN** the fleet board reflects all agents from `/api/digest`, and a message
  (typed or a quick-command chip) is delivered to the supervisor session

#### Scenario: Selecting a fleet row targets a command

- **WHEN** the user taps a fleet row
- **THEN** it is selected and per-target quick actions (e.g. continue / inspect /
  reply-for-me) become available in the command bar; a long-press instead jumps
  to that worker's own detail
