## ADDED Requirements

### Requirement: The Detail terminal shows Codex's pinned prompt in full

Codex pins the prompt of the turn on screen to row 0 as one row: "› ", the prompt with its
newlines joined, cut to the pane's width and ended with "…", kept after the turn ends. When a
Codex pane's capture starts with such a row — ending within two cells of the pane's width
(the server's `cols`) — and a later row begins with "› " (the composer), and exactly one of
the conversation's ten most recent logged prompts is longer than the row's text and begins
with it (ignoring whitespace), and the row below does not carry on with that same prompt (a
history message wrapped rather than cut), the Detail terminal SHALL show that full prompt in
a bar at the bottom of the floating chrome and SHALL render the capture without the row. A
prompt still being sent from the phone SHALL NOT be a candidate. The bar SHALL show two lines
at rest, open to the whole prompt on a tap, and copy it on a long press; its screen-reader
label SHALL carry at most the first 160 characters, as this turn's prompt. Its height SHALL be
part of the terminal's top padding and of the distance the chrome slides out. In any other
case — another agent, no width from the server, a row short of the edge, no match, two
matching prompts, no composer row, full screen — the terminal SHALL render the capture exactly
as received, and the Chat view SHALL NOT show the bar. While the row is on screen in the
terminal and no prompt explains it, the terminal SHALL refetch the conversation log, at most
once every 4 seconds, and stop once it is explained.

#### Scenario: A cut Codex prompt

- **WHEN** a Codex pane's top row reads "› 你是独立只读诊断 worker … 任务：核实…" to the pane's
  right edge and the conversation's latest prompt begins with that text
- **THEN** the bar shows the whole prompt and the terminal starts at the row below it

#### Scenario: Another agent shows the same bytes

- **WHEN** a Claude Code pane's capture is identical
- **THEN** the terminal renders it unchanged and no bar appears

#### Scenario: The prompt is not known yet

- **WHEN** no recent prompt matches the cut row
- **THEN** the terminal shows the cut row as captured, and refetches the log until it matches

#### Scenario: A new turn under an unchanged status

- **WHEN** Codex moves from prompt A to prompt B while staying working, and B reaches its log
  after the terminal last read it
- **THEN** the bar disappears, the cut row shows as captured, and the bar returns with B
  within one refetch of B being logged

#### Scenario: Two prompts open the same way

- **WHEN** two different recent prompts both begin with the row's text
- **THEN** the terminal shows the cut row as captured and no bar appears

#### Scenario: The user's own "…"

- **WHEN** row 0 is a prompt that ends with the user's own "…" short of the right edge, or
  is no longer than the row, or continues on the next row
- **THEN** the terminal shows it as captured and no bar appears

#### Scenario: Full screen

- **WHEN** the user enters full screen on a Codex pane with a pinned prompt
- **THEN** the cut row stays in the terminal and no bar is shown

## REMOVED Requirements

### Requirement: A phone may preserve the Mac pane's original rows

**Reason**: The Original width mode could not show what it was built for: Codex cuts its
pinned prompt before tmux sees the rest, so no canvas width brings it back. The user found the
toggle useless; the pinned prompt bar replaces it.

**Migration**: None. The terminal wraps to the device, as it did by default; `GET /api/pane`
still reports `cols` for other clients.
