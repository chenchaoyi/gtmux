## ADDED Requirements

### Requirement: The Detail terminal shows Codex's pinned prompt in full

When a Codex pane's captured screen starts with Codex's pinned prompt row — row 0 beginning
with "› " and cut with "…" within three rows — and the text before the "…" begins one of the
conversation's recent prompts (ignoring whitespace), and a later row begins with "› ", the
Detail terminal SHALL show that full prompt in a bar at the bottom of the floating chrome and
SHALL render the capture without the pinned rows. The bar SHALL show two lines at rest, open
to the whole prompt on a tap, and copy it on a long press. Its height SHALL be part of the
terminal's top padding and of the distance the chrome slides out. In any other case — another
agent, no match, no composer row, full screen — the terminal SHALL render the capture exactly
as received, and the Chat view SHALL NOT show the bar.

#### Scenario: A cut Codex prompt

- **WHEN** a Codex pane's top row reads "› 你是独立只读诊断 worker … 任务：核实…" and the
  conversation's latest prompt begins with that text
- **THEN** the bar shows the whole prompt and the terminal starts at the row below it

#### Scenario: Another agent shows the same bytes

- **WHEN** a Claude Code pane's capture is identical
- **THEN** the terminal renders it unchanged and no bar appears

#### Scenario: The prompt is not known yet

- **WHEN** no recent prompt matches the cut row
- **THEN** the terminal shows the cut row as captured

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
