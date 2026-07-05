## ADDED Requirements

### Requirement: Menu bar shows a distinct native-sessions category
The menu-bar popover SHALL group `source: "native"` sessions under their own labelled section (e.g. "Elsewhere" / "不在 tmux"), separate from the tmux-based needs-you / working / idle groups, so users can see these sessions exist and their rough info (agent, project, state, idle time) without implying they can be jumped to or replied to.

#### Scenario: Native section rendered when native sessions exist
- **WHEN** the app polls `agents --json` and native sessions are present
- **THEN** they SHALL appear in a dedicated, clearly-labelled category distinct from the tmux groups

#### Scenario: Native rows expose no jump or reply affordance
- **WHEN** a native row is rendered
- **THEN** it SHALL NOT show a jump chevron or a reply/send control, and clicking it SHALL NOT attempt a terminal focus

### Requirement: Move-to-tmux action in the menu bar
The menu bar SHALL provide a "Move to tmux" action on an eligible native row that resumes that conversation in a fresh tmux session. The action SHALL be shown only for a row that is movable (idle, resumable, with an on-disk conversation), and SHALL surface a confirmation explaining that the original process is exited before acting.

#### Scenario: Move a native session
- **WHEN** the user triggers Move to tmux on a movable native row and confirms
- **THEN** the app SHALL invoke the resume/spawn path to open a tmux session running that conversation

#### Scenario: Move hidden for ineligible rows
- **WHEN** a native row is not movable (working, non-resumable, or no on-disk conversation)
- **THEN** the Move to tmux action SHALL NOT be offered for that row
