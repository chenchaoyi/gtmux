## ADDED Requirements

### Requirement: Saved completion alone does not occupy the live Codex overview
A Codex observation with no recorded process ID, an idle derived status, and no explicit HQ follow SHALL be omitted from live radar rows. This presentation filter SHALL NOT remove its observation, saved history or consent boundary. Working, waiting, process-backed and explicitly followed observations SHALL retain their existing eligibility rules. A fresh accepted turn SHALL make an otherwise eligible observation visible again. DSH saved-session discovery SHALL remain unchanged.

#### Scenario: Completed legacy observation
- **WHEN** a PID-less Codex observation's same-session saved turn completes and it is not followed
- **THEN** it disappears from the live overview without being declared closed or deleting its observation or conversation

#### Scenario: Active or followed observation
- **WHEN** the observation is working or waiting, has a recorded process ID, or has explicit HQ follow
- **THEN** this presentation filter does not hide it

### Requirement: Detached Codex hooks retain bounded process identity
Before detaching a Codex hook, the system SHALL capture the nearest recognized agent ancestor's PID, executable basename and process birth using only narrow process metadata. The walk SHALL have depth, time and output bounds and SHALL revalidate the sampled chain. Its private worker handoff SHALL reject malformed, expired or future evidence. Unknown evidence SHALL remain unknown, never the detached worker's own identity. Native records with an exact birth SHALL compare it during liveness checks; a changed birth SHALL prove PID reuse, and an unreadable birth SHALL receive only the existing unknown-process grace. Process identity SHALL grant no frontend or input authority.

#### Scenario: Native agent exits
- **WHEN** a Codex hook is detached, records the original agent process, and that process later exits
- **THEN** the native record is pruned by the existing dead-process path instead of waiting for the PID-less grace

#### Scenario: Process identity changes or cannot be read
- **WHEN** the ancestry changes while sampled, metadata exceeds its bounds, or handoff evidence is stale
- **THEN** no process identity is claimed
