## ADDED Requirements

### Requirement: The pane view shows Codex's pinned prompt in full

In the single-pane terminal view, when a Codex pane's capture starts with Codex's pinned prompt
row — "› ", the prompt with its newlines joined, cut at the pane's width and ended with "…",
ending within two cells of the pane's `cols` — and a later row begins with "› " (the
composer), and exactly one of the conversation's ten most recent logged prompts is longer than
the row's text and begins with it (ignoring whitespace), and the row below does not carry on
with that same prompt, the browser mirror SHALL show that full prompt in a bar above the
terminal and SHALL write the capture to the terminal without the row. The bar SHALL show two
lines at rest, open to the whole prompt on a click, and offer Copy. Cell widths SHALL be the
phone's tmux-measured widths. In any other case — another agent, no `cols`, a row short of the
edge, no match, two matching prompts, no composer row, chat mode, a workbench tile — the capture
SHALL be written exactly as received and no bar shown. While the row is on screen and no prompt
explains it, the view SHALL fetch the conversation log at most once every 4 seconds, and stop
once it is explained.

#### Scenario: A cut Codex prompt in the browser

- **WHEN** a Codex pane's top row is cut at its right edge and the latest logged prompt begins
  with it
- **THEN** the bar shows the whole prompt and the terminal starts at the row below it

#### Scenario: Another agent's identical screen

- **WHEN** a Claude Code pane shows the same bytes
- **THEN** the terminal shows them unchanged and no bar appears

#### Scenario: The prompt is logged late

- **WHEN** Codex moves to a new prompt that its log does not hold yet
- **THEN** the cut row shows as captured, and the bar returns within one fetch of the prompt
  being logged

#### Scenario: The same rules as the phone

- **WHEN** the shared case fixture runs against the browser's matcher and the phone's
- **THEN** both give the same answer for every case
