# agent-dispatch — delta

## ADDED Requirements

### Requirement: A dispatch blocked at a startup gate or holding an undelivered draft is never done

The system SHALL classify a dispatched worker that is blocked BEFORE running a turn —
sitting at a startup/permission gate, or holding its pasted-but-unsubmitted goal in the
composer — as `waiting` (needs-you) in `gtmux tasks` and the digest, and SHALL NEVER
report it as `done`. `done` SHALL be reserved for a dispatch whose session actually
completed a turn. The system SHALL also surface WHY via a kind (`startup` / `draft`).

#### Scenario: `gtmux tasks` shows a stuck dispatch as waiting, not done

- **WHEN** a dispatch's pane is at a startup gate or still holds its undelivered draft
- **THEN** `gtmux tasks` and the digest show it `waiting` (needs-you), not `done`, so a
  supervisor is never told a task finished when not one step ran
