## MODIFIED Requirements

### Requirement: Read-only grouped health check

During the agent and notification section, `gtmux doctor` SHALL announce each
subcheck before running it when progress is enabled. The last line SHALL name
the currently running hook, chat-binding, or hook-traffic probe.

#### Scenario: Agent probe is slow

- **WHEN** a chat-binding or hook-traffic probe takes a long time
- **THEN** stderr identifies that probe without changing the report on stdout
