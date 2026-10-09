## ADDED Requirements

### Requirement: Mobile layout bounds prompt and radar chrome

The app SHALL keep the matched Codex instruction entrance compact and open its full text in a separate safe-area reading sheet, without changing terminal top padding on expansion. Radar floating-disc clearance SHALL be bounded independently of list content height; the footer SHALL explain rows hidden by collapsed sections.

#### Scenario: Long instruction and collapsed radar

- **WHEN** the user opens a long instruction or folds a radar section
- **THEN** the instruction reader scrolls independently, the terminal is not covered by expanded chrome, and the radar footer explains the hidden rows with bounded bottom clearance
