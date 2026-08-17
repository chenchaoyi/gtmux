# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: HQ self-check and self-maintenance

The seeded playbook SHALL teach HQ, on a gtmux-raised self-check trigger — delivered as a
STANDING-priority wake line (`» gtmux·self-check …`) and recorded as a
`[CONTROL gtmux:self-check]` entry in the session-event stream — to review and maintain
its OWN artifacts: event-log health, attention-ledger hygiene (settle stale
pending entries), memory/knowledge-base quality, and accumulated low-value
items, using only its existing write-own-notes authority. HQ SHALL default to SILENT
self-maintenance, printing a one-line brief ONLY when it took a real action, and SHALL
escalate a severe finding (rotation broken, sequence gap, mass-invalid memory) as
CRITICAL. The self-check sensor's own control records SHALL NOT be counted as recent
user-facing attention, so a raised trigger cannot suppress the idle condition that
raised it.

#### Scenario: Silent maintenance when nothing needed

- **WHEN** a self-check trigger fires and HQ finds nothing to fix
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real cleanup is briefed in one line

- **WHEN** a self-check trigger fires and HQ settles stale pending entries or prunes
  stale memory
- **THEN** HQ prints a single one-line brief of what it did

#### Scenario: A severe finding escalates

- **WHEN** a self-check finds a broken rotation, a sequence gap, or mass-invalid memory
- **THEN** HQ surfaces it as CRITICAL rather than quietly cleaning up

#### Scenario: A raised self-check does not suppress the next one

- **WHEN** a self-check trigger has been raised into an otherwise quiet fleet and the idle
  window elapses again
- **THEN** the idle trigger still fires — gtmux's own control record is not read as the
  user having been recently pinged
