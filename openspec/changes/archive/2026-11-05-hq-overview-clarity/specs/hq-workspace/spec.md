## ADDED Requirements
### Requirement: Overview previews disclose remaining goals and current state
The native HQ overview SHALL expose the exact remaining count at the bottom of each bounded section and open the existing complete list. Goal state SHALL come from canonical records, separately from worker status.
#### Scenario: More goals exist
- **WHEN** a section contains more goals than its visible preview
- **THEN** the footer names the remaining count and opens all goals without sending an instruction
#### Scenario: A stage result arrives
- **WHEN** a goal remains open after a stage result
- **THEN** the overview keeps its unfinished status and does not infer whole-goal completion
