# notifications — delta

## ADDED Requirements

### Requirement: The supervisor is a meta-layer in notifications

The supervisor (HQ) session SHALL NOT be treated as a normal worker in the notification,
push, and lockscreen layers. The fleet tally that drives the lockscreen (the
Waiting/Working/Idle counts and the "who's waiting" headline) SHALL exclude the
supervisor, so HQ never inflates the worker counts nor hijacks the headline. The hook
SHALL suppress the supervisor's routine `done` notification (a supervisor finishing a
think-cycle must not notify the user); the supervisor's `input` notification (it needs a
decision from the user) SHALL be kept.

#### Scenario: HQ does not pollute the worker tally

- **WHEN** the serve fleet snapshot includes a `role:"supervisor"` pane that is waiting
- **THEN** the lockscreen tally's waiting count and "who's waiting" headline are computed from the worker panes only — the supervisor is excluded

#### Scenario: HQ's routine completion is silent

- **WHEN** the supervisor session finishes a turn (a `done`/Stop event) with the user not viewing it
- **THEN** no `done` notification is posted for it (unlike a worker), because a chief-of-staff completing a think-cycle is routine noise

#### Scenario: HQ still reaches you when it needs a decision

- **WHEN** the supervisor session emits an `input`/Waiting event (it needs the user's decision)
- **THEN** a notification is still posted, since that is the one thing the supervisor should surface
