## MODIFIED Requirements

### Requirement: HQ rotates itself through gtmux, not through raw tmux

`gtmux hq --rotate` SHALL durably queue a request for the current HQ pane, agent, and
retiring session ID. It SHALL report pending rather than success. The resident process
SHALL wait for completion evidence from the same session and a confirmed empty input
box before sending the agent's reset command once. It SHALL NOT overwrite a draft or
type during copy mode. A distinct observed successor session ID SHALL be required for
a successful `gtmux:audit:rotate` record with both identities. Rejected or timed-out
attempts SHALL leave a `gtmux:audit:rotate-failed` record and SHALL NOT be retried
automatically. A duplicate request for the same session SHALL remain one request.

#### Scenario: Active Codex HQ requests rotation

- **WHEN** the current Codex rollout ends in `task_started` even if its composer is visible
- **THEN** the request is queued without sending `/new`; after `task_complete` and a safe
  composer, the resident process submits it once and waits for a distinct session ID

#### Scenario: An unsafe or unsuccessful reset

- **WHEN** the composer has a draft, the pane is in copy mode, or the reset yields no
  observable successor before the deadline
- **THEN** no successful rotation is claimed, and the request waits or fails with an
  explicit audit reason without a blind second send
