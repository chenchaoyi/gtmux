## ADDED Requirements

### Requirement: Mark errored idle rows in the popover

The menu-bar popover SHALL visually distinguish an idle agent that ended on an
error (`error: true` in the `agents --json` contract) from a successfully-finished
idle agent, using an amber ⚠ "errored" modifier and the `error_text` summary in
place of the green ✓. The row SHALL remain in the IDLE section and MUST NOT use the
red `waiting`/needs-you color.

#### Scenario: Errored idle agent

- **WHEN** an agent row has `status: idle` and `error: true`
- **THEN** the popover renders it in the IDLE section with an amber ⚠ marker (not
  the green ✓) and shows the `error_text` summary
- **AND** it is not colored red and does not sort into NEEDS YOU

#### Scenario: Successful idle agent unchanged

- **WHEN** an agent row has `status: idle` without `error`
- **THEN** the popover renders it exactly as today (green ✓)
