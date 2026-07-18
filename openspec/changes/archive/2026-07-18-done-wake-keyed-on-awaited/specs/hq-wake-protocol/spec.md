# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Unattended completions wake HQ immediately with a judgment payload

A turn-end that leaves a session idle SHALL fire an immediate `done` wake when the
session's pane is NOT the focused pane of an attached tmux client at that moment
(the human was not watching it finish), OR when the pane is AWAITED — a pane HQ
dispatched work to (via `gtmux spawn` or `gtmux send`) and is expecting a completion
from. An AWAITED completion SHALL fire the immediate wake EVEN when the pane is the
focused pane of an attached client, overriding the attended-defer, because HQ is
explicitly waiting on that pane and must not be left to a deferred tick. The wake line
SHALL carry enough to judge without a drill: pane id and location, turn duration, the
session's goal, and the reply tail summary, each marked as DATA. A completion in the
focused pane of an attached client that is NOT awaited SHALL NOT fire an immediate wake
and SHALL count toward the tick tally instead. The awaited flag SHALL be cleared once
the wake fires (a one-shot per dispatch). The behavior SHALL remain configurable
(`hqWake.done`: `unattended` default | `always` | `tick`); the awaited override applies
under the `unattended` default (the case the bug occurs in).

#### Scenario: A background session finishes

- **WHEN** a session in an unfocused window reaches idle after work
- **THEN** HQ receives one `done` wake line carrying loc, duration, goal, and the
  reply tail

#### Scenario: A completion under the user's eyes stays quiet

- **WHEN** a session finishes in the pane the user currently has focused in an
  attached client, and the pane is NOT awaited
- **THEN** no immediate wake fires and the completion is included in the next tick
  brief

#### Scenario: An HQ-awaited completion wakes immediately even when attended

- **WHEN** HQ drove a pane via `gtmux send` (or `spawn`) — so the pane is awaited — and
  that pane finishes while it is the focused pane of an attached client
- **THEN** HQ receives an immediate `done` wake on the acked wake channel (not a
  deferred tick tally), and the awaited flag is cleared
