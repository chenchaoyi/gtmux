## ADDED Requirements

### Requirement: Fixed full-screen chat controls remain inside the safe area

The phone and iPad full-screen chat fold controls SHALL use the raw top safe-area
inset and a horizontal gutter independently of scrolling content. Their label and
touch target SHALL clear device corners and the Dynamic Island. Normal-mode flow,
horizontal safe-area handling and collapse/expand actions SHALL be preserved.

#### Scenario: Full-screen chat on a phone with rounded corners

- **WHEN** the user enters full-screen chat with a nonzero top safe-area inset
- **THEN** the fixed Collapse all / Expand all control floats below that inset,
  while content can still scroll across the full viewport

#### Scenario: Normal mode or landscape

- **WHEN** full-screen is off, or the top inset becomes zero in landscape
- **THEN** normal controls retain their flow layout; full-screen controls use the
  current zero top inset inside the existing horizontal safe area
