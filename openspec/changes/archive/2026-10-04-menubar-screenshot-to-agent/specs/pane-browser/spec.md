## ADDED Requirements

### Requirement: Panes report when a terminal last showed them

`gtmux panes --json` (and `GET /api/panes`) SHALL carry an additive, optional `viewed_at`
on each pane that an attached tmux client is currently showing: the newest
`client_activity` among those clients, in unix seconds. A pane no client shows SHALL omit
it.

#### Scenario: Two terminals on two panes

- **WHEN** one terminal shows %1 and was used after another terminal showing %2
- **THEN** %1's `viewed_at` is newer than %2's, and a pane neither shows has none
