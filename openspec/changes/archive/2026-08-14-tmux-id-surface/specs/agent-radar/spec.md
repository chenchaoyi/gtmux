# agent-radar (delta)

## ADDED Requirements

### Requirement: The pane producer captures the stable window tmux id

`panes --json` SHALL additively capture the tmux `window_id` (`@N`) as `win_id`, with
`window_name` as `win_name`, so a window can be grouped and addressed by an id that
survives renaming and reordering, not only by its mutable index. Both SHALL be optional
and additive: absent fields stay absent, and the fields SHALL be appended to the END of
the tmux format string so an index-based parser is undisturbed.

`agents --json` SHALL NOT carry these fields. The radar is a flat list of agents with no
window level, so a window id there would be a speculative field with no consumer; it is
added when one exists, not for symmetry.

The new tmux-identity fields SHALL be named so they do NOT collide with the existing
`session_id` JSON field, which is the coding-agent ADOPT key (populated for `source:
"native"` rows) and is NOT tmux's `$N` — hence `win_id`, not `window_id`.

#### Scenario: A renamed session keeps its panes' window identity

- **WHEN** a session is renamed or its windows are reordered
- **THEN** each pane's captured `@<window_id>` is unchanged, so a consumer grouping by
  window id sees the same windows

#### Scenario: The adopt key is not overloaded

- **WHEN** a consumer reads the tmux window id and the coding-agent adopt id
- **THEN** they are distinct JSON fields; the tmux id does not overwrite or alias the
  existing `session_id` adopt key

#### Scenario: Two windows that both have index 0 are still distinct

- **WHEN** two sessions each hold a window at index `0`
- **THEN** their panes carry different `win_id`s, so a consumer grouping by window id does
  not merge them — the measured failure that the index alone could not tell apart
