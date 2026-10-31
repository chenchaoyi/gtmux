## ADDED Requirements

### Requirement: Complete compact code and structured working-tree Diff
The browser SHALL load complete selected tool input on reader intent with truthful loading/failure/retry states and original copying. Code formatting and Diff structure SHALL share the native model. Working-tree Diff SHALL present file cards, file and hunk counts, line ranges and old/new line numbers with independent disclosure and syntax colors. Long hunks SHALL mount in readable batches and narrow layouts SHALL wrap without widening the page.

#### Scenario: Context and code are read in the same trace
- **WHEN** the reader switches from a tool call to context Raw content without a code-language field
- **THEN** the content stays readable and exact without a rendering exception
- **AND** system shortcuts, historical viewport and private-scope retirement remain intact
