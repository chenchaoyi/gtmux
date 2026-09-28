## MODIFIED Requirements

### Requirement: Codex turn completion is attributed to its own session and pane

The active marker MAY omit a session ID when Codex omits it on `UserPromptSubmit`.
The Stop resolver and radar SHALL accept such a marker only for a live pane with
a bound Codex conversation whose latest rollout event is `task_complete` after
the marker's creation. A marker naming another session, an older completion,
or multiple eligible panes SHALL not be claimed.

#### Scenario: Plain active marker and ownerless Stop

- **WHEN** a Codex turn has a plain active marker and the bound rollout completes
  after the marker, while its Stop hook lacks pane identity
- **THEN** the hook or next radar read marks that pane idle without changing any peer
