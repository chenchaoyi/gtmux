# menu-bar-app Specification

## ADDED Requirements

### Requirement: Popover width sized for content legibility

The menu-bar popover SHALL use a fixed content width (a single design token,
`Theme.Size.popoverWidth`) wide enough that the digest text — the HQ card's
goal/last/ask line and each agent row's session/task line — is legible before
tail-truncation. The width SHALL be **420pt**, matching the MPBar companion so the
two menu-bar apps read as one visual family. Every row SHALL inherit this width (via
the popover frame or `maxWidth: .infinity`); no per-row width may be hardcoded.

Long content SHALL be handled by single-line tail-truncation, NOT by reflowing the
popover: the goal/last/ask and session/task lines SHALL remain
`lineLimit(1)` + `truncationMode(.tail)` at any width, so the wider frame only reveals
more text and never changes the number of lines.

The width SHALL be a fixed constant rather than content-adaptive: because every row is
single-line tail-truncated, no row's content requires a wider frame, so an adaptive /
max-width popover would add width jitter with no legibility gain.

#### Scenario: Popover renders at the calibrated width

- **WHEN** the popover is shown
- **THEN** its content frame is 420pt wide
- **AND** the width comes from the single `Theme.Size.popoverWidth` token, not a
  per-row constant

#### Scenario: A long goal/last/ask line truncates rather than reflows

- **WHEN** an HQ card or agent row carries a goal/last/ask or session/task string
  longer than the row can show
- **THEN** the string is shown on one line, tail-truncated with an ellipsis
- **AND** the wider frame reveals more of the string but does not add a second line
  or change the popover width
