## ADDED Requirements

### Requirement: Completed checks distinguish result severity
Doctor SHALL retain its four result classes: passed prerequisites, recommendations,
blocking conditions affecting their corresponding features, and information.
Completed reports SHALL state completion rather than labelling findings as checks
still pending. Information SHALL NOT count as an actionable finding. A read-only
recheck SHALL NOT claim to repair findings. Check failure SHALL remain distinct
from an otherwise complete report containing recommendations.

#### Scenario: Recommendations remain after rechecking
- **WHEN** a completed read-only check returns two recommendations and no blocking conditions
- **THEN** it reports two recommendations after completion, not two pending checks
- **AND** its health exit code remains zero

### Requirement: Findings are summarized with their provenance and guidance
The human-readable doctor report SHALL end with every blocking and recommended
finding, blocking first, retaining each check's group, current value and guidance.
The fix pass SHALL summarize remaining findings without treating every
recommendation as a failed repair. The Mac report SHALL make the same findings
accessible without searching all groups, and provide navigation to each original
check and its details. No informational or passed row SHALL appear in this list.

#### Scenario: Flagged rows are spread across groups
- **WHEN** a report contains findings in Logs and HQ
- **THEN** both appear in the final CLI summary and the Mac findings summary
- **AND** the Mac summary can locate their original rows

#### Scenario: No actionable findings
- **WHEN** all rows are passed or informational
- **THEN** the report states completion without a pending repair list
