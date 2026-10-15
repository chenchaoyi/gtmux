## ADDED Requirements

### Requirement: Operation-owned Codex path discovery

A complete digest or usage producer SHALL share one lazy Codex transcript path-discovery
scope across gathering, structured content and per-session accounting. Ordered HQ
sensors MAY share the same path scope, but SHALL collect fresh rows at each step.
Scopes SHALL NOT persist between requests, cache transcript/status/permission facts,
bypass driver content switches or change the base rollout used for usage byte offsets.

#### Scenario: Digest contains multiple Codex sessions

- **WHEN** a digest gathers two Codex sessions and then reads their content and usage
- **THEN** transcript candidate discovery happens once for that operation
- **AND** the next operation discovers new rollouts and reads current content

#### Scenario: A sensor changes a marker between gathers

- **WHEN** ordered sensors reuse a collection after an earlier sensor changes state
- **THEN** the next gather sees the changed state while reusing only path discovery
