# agent-dispatch — delta

## ADDED Requirements

### Requirement: Standardized spawned-window title and live locator handle

`gtmux spawn` SHALL name a spawned window by its PURPOSE — a concise title slug (from
`--title`, else derived) applied as the window and pane name — and SHALL report the
window's tmux number as a LIVE locator, never a value baked into the name (so it stays
correct under `renumber-windows`). On a successful dispatch `spawn` SHALL emit the
standard handle `<loc> (%pane) · <title>`, where `loc` is the live `session:window.pane`
and `title` is the purpose slug; `--json` SHALL include `loc` and `title` fields. The
supervisor's playbook SHALL require a concise `--title` on every dispatch and this handle
in every report, so a spawned window can be referred to and jumped to by its number.

#### Scenario: The report exposes the live window number and purpose

- **WHEN** `gtmux spawn --title fix-auth-mw <goal>` lands
- **THEN** it reports `<session>:<window>.<pane> (%id) · fix-auth-mw`, with the window number read live, and `--json` carries `loc` and `title`

#### Scenario: The window number is not baked into the name

- **WHEN** a spawned window's session later has an earlier window closed under `renumber-windows on`
- **THEN** the reported/derivable window number reflects the CURRENT tmux index (the locator is read live), and the window's name remains the purpose title with no stale number in it
