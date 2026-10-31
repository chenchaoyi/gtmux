## ADDED Requirements

### Requirement: Compact complete code and structured Diff reading
Native Chat and Trace SHALL format supported code for compact reading while preserving original source copying and fallback for unsupported or incomplete code. A code reader SHALL identify its language and allow original-text reading. Session text size SHALL adjust one point at a time, preserve legacy actual sizes and apply to Chat and Terminal. Diff SHALL expose working-tree file cards, actual change counts, independently folded hunks, old/new line numbers, syntax colors and complete original file patches while preserving bounded row mounting and scroll indicators.

#### Scenario: Read one file while other change regions are folded
- **WHEN** a reader expands a file or hunk in a large Diff
- **THEN** only that disclosure changes and unchanged source remains available in the complete file reader
- **AND** wrapped code fits the narrow viewport, with original-width reading still available
