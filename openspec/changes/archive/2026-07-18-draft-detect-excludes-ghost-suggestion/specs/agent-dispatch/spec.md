# agent-dispatch (delta)

## MODIFIED Requirements

### Requirement: A dispatch blocked at a startup gate or holding an undelivered draft is never done

The system SHALL classify a dispatched worker that is blocked BEFORE running a turn —
sitting at a startup/permission gate, or holding its pasted-but-unsubmitted goal in the
composer — as `waiting` (needs-you) in `gtmux tasks` and the digest, and SHALL NEVER
report it as `done`. `done` SHALL be reserved for a dispatch whose session actually
completed a turn. The undelivered-draft state SHALL be judged from a COLOR-aware capture
that EXCLUDES the agent's suggested-next-command GHOST text — the dim autosuggestion the
agent renders faint (SGR 2), which needs a key to accept and is NOT user input — so a
composer showing only a ghost suggestion is NOT read as an undelivered draft. The system
SHALL also surface WHY via a kind (`startup` / `draft`).

#### Scenario: `gtmux tasks` shows a stuck dispatch as waiting, not done

- **WHEN** a dispatch's pane is at a startup gate or still holds its undelivered draft
- **THEN** `gtmux tasks` and the digest show it `waiting` (needs-you), not `done`, so a
  supervisor is never told a task finished when not one step ran

#### Scenario: A dim suggested-next-command does not block `done`

- **WHEN** a dispatch's pane completed its turn and its composer shows only the agent's
  faint suggested-next-command ghost text (SGR 2), with no real unsubmitted input
- **THEN** the ghost text is not read as an undelivered draft, so the completion is NOT
  suppressed as a stuck `draft`
