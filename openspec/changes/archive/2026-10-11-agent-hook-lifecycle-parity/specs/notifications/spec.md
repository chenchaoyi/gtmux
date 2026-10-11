## ADDED Requirements

### Requirement: Polling completion alerts exclude terminal cancellation and failure

The independent server alert poller SHALL retain the hook or rollout outcome across radar reads. A working-to-idle transition caused by interruption or terminal failure SHALL update shared agent state without emitting a successful completion alert. A later actual new turn that completes normally SHALL remain eligible for the usual completion alert.

#### Scenario: Cancellation followed by normal completion

- **WHEN** an active turn is interrupted or fails and the independent poller observes idle
- **THEN** it emits the agent-state update without a done alert
- **AND** a subsequent real turn ending successfully emits the normal done alert
