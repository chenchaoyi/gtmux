## ADDED Requirements

### Requirement: Antigravity rows resolve saved conversation titles and strip binary-name placeholders

The radar SHALL strip generic `agy`, `antigravity`, and `antigravity-cli` placeholder terminal titles from Antigravity panes, and SHALL populate `Task` for verified bound tmux panes and native rows from `$AGY_HOME/annotations/<conversation-id>.pbtxt` when a saved conversation title exists.

#### Scenario: Bound Antigravity pane displays its renamed or summarized conversation title

- **WHEN** an Antigravity tmux pane has `pane_title` `"agy"` and a verified resume record bound to a conversation whose `$AGY_HOME/annotations/<id>.pbtxt` contains `title:"gtmux agy dev"`
- **THEN** the radar row reports `Task` as `"gtmux agy dev"`

#### Scenario: Unbound or untitled Antigravity pane strips the binary-name placeholder

- **WHEN** an Antigravity tmux pane has `pane_title` `"agy"` and no saved annotation title
- **THEN** the radar row reports `Task` as `""` so surfaces fall back to the tmux session name rather than `"agy"`
