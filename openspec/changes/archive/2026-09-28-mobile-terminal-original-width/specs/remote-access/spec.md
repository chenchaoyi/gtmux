# remote-access (delta)

## ADDED Requirements

### Requirement: Pane snapshots report their source width

`GET /api/pane` and successful `POST /api/send` pane snapshots SHALL report the
source tmux pane's column count when available, without changing the existing
text and cursor fields. The field SHALL be absent when tmux cannot report it.

#### Scenario: A wide pane is captured

- **WHEN** the server captures a pane that is 189 columns wide
- **THEN** its response includes `cols: 189` alongside the captured text
