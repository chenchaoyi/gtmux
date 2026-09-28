# agent-radar (delta)

## ADDED Requirements

### Requirement: Native Codex rows use saved conversation titles

For a Codex session sensed outside tmux, the radar SHALL put its saved
`thread_name` from Codex's session index into the existing `task` field only when
the index entry's ID equals that native session's ID. The latest entry for an ID
SHALL win. If the title is missing, empty, or unreadable, `task` SHALL remain empty
so clients retain their project/terminal fallback. The radar MUST NOT use a
conversation prompt as a substitute title, and title lookup MUST NOT change the
native row's state, identity, or eligibility to move into tmux.

#### Scenario: Renamed native Codex session

- **WHEN** Codex's index has multiple title entries for the same live native session
- **THEN** that row's `task` contains the last saved title, and no other row inherits it

#### Scenario: No saved title

- **WHEN** the index is absent or has no nonempty title for a native session
- **THEN** the row's `task` is empty and clients use their existing fallback label
