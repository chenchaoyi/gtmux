## ADDED Requirements

### Requirement: Diff scroll indicators stay inside the visible reading viewport
Phone and iPad SHALL show light vertical scroll indicators on the dark Diff canvas while scrolling. When long lines widen the nested vertical list, its indicator SHALL remain at the visible viewport edge across horizontal positions and viewport changes. Horizontal indicators SHALL also contrast with the canvas. Row virtualization, bounded line measurements and recorded content SHALL remain intact.

#### Scenario: A wide diff is panned horizontally
- **WHEN** a reader scrolls vertically with the horizontal view at its left, middle or right position
- **THEN** the vertical indicator remains visible at the current viewport edge rather than the long-line content edge
- **AND** changing the viewport or reading appearance does not hide it or mount the whole diff

## MODIFIED Requirements

### Requirement: Structured transcript activity

Phone, iPad and Web SHALL display each transcript tool-step segment as a neutral disclosure with an activity heading and total call count. Expanded groups SHALL preserve call order and bounded argument summaries. Phone/iPad SHALL use an unboxed grey summary and compact rows with distinct neutral semantic icons for known actions, with a generic icon for unknown tools. Only rows with additional recorded input, directory or a partial-detail notice SHALL offer detail expansion; empty inspectors SHALL be omitted. Details SHALL retain the recorded tool name, one selected original input and its recorded directory; long command lines SHALL scroll horizontally without wrapping, multiline content SHALL use a bounded vertical viewport, and whole-group collapse SHALL remove row/detail content. Copy SHALL copy only displayed input. Partial input and summary-only older cores SHALL use a visible, accessible information icon with an on-demand explanation. Overflowing input SHALL offer a compact draggable horizontal scrollbar within the existing header, synchronized with native code scrolling and accessibility adjustment actions; helper paragraphs SHALL NOT occupy the input body. Web MAY retain its existing tool-count summary and wrapping argument previews.

Codex summaries SHALL prefer non-empty string `title`, then `description`, before existing command/path fallbacks, retaining the existing 80-rune limit. Surfaces SHALL render summaries as plain text and SHALL NOT infer per-call success, completion, duration or output from the existence of a call. Phone/iPad live and historical groups SHALL start folded until the reader opens them, and explicit reader choices SHALL survive updates and dropped older turns.

#### Scenario: Repeated browser calls

- **WHEN** a segment records six `js` calls with descriptions and one `wait`
- **THEN** its group counts seven calls, summarizes the repeated tools, and shows the recorded descriptions when expanded
- **AND** no call is labelled successful from its presence in the log

#### Scenario: An older core provides only tool names

- **WHEN** steps have no argument summaries
- **THEN** their ordered tool identities remain readable and the group can still be folded

#### Scenario: Inspect a long command on phone or iPad

- **WHEN** a reader expands a recorded command row
- **THEN** its original line breaks and argument text are selectable and scrollable horizontally, with the actual tool name and any recorded directory
- **AND** folding the row or whole group removes the detail, while appended calls do not override the reader's group choice
- **AND** a limited input is labelled partial, an old-core preview is labelled summary-only, and no result or duration is invented
