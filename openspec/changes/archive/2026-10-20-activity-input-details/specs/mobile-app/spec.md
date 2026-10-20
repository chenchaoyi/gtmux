## MODIFIED Requirements

### Requirement: Structured transcript activity

Phone, iPad and Web SHALL display each transcript tool-step segment as a neutral disclosure card with an activity heading and total call count. Expanded cards SHALL preserve call order and retain the recorded tool name and bounded argument summary. Phone/iPad SHALL use a compact summary row and distinct neutral semantic icons for known actions, with a generic icon for unknown tools. Individual rows SHALL disclose one selected original input and its recorded directory; long command lines SHALL scroll horizontally without wrapping, multiline content SHALL use a bounded vertical viewport, and whole-group collapse SHALL remove row/detail content. Copy SHALL copy only displayed input. Partial input and summary-only older cores SHALL be labelled. Web MAY retain its existing tool-count summary and wrapping argument previews.

Codex summaries SHALL prefer non-empty string `title`, then `description`, before existing command/path fallbacks, retaining the existing 80-rune limit. Surfaces SHALL render summaries as plain text and SHALL NOT infer per-call success, completion, duration or output from the existence of a call. The phone/iPad running turn SHALL keep its existing open-by-default policy, completed history SHALL stay folded, and explicit reader choices SHALL survive updates and dropped older turns.

#### Scenario: Repeated browser calls

- **WHEN** a segment records six `js` calls with descriptions and one `wait`
- **THEN** its card counts seven calls, summarizes the repeated tools, and shows descriptions beside the actual tool names when expanded
- **AND** no call is labelled successful from its presence in the log

#### Scenario: An older core provides only tool names

- **WHEN** steps have no argument summaries
- **THEN** their ordered tool names remain readable and the card can still be folded

#### Scenario: Inspect a long command on phone or iPad

- **WHEN** a reader expands a recorded command row
- **THEN** its original line breaks and argument text are selectable and scrollable horizontally, with the actual tool name and any recorded directory
- **AND** folding the row or whole group removes the detail, while appended calls do not override the reader's group choice
- **AND** a limited input is labelled partial, an old-core preview is labelled summary-only, and no result or duration is invented
