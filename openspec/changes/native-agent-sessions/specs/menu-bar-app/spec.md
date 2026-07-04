## ADDED Requirements

### Requirement: Menu bar shows a distinct native-sessions category
The menu-bar popover SHALL group `source: "native"` sessions under their own labelled section (e.g. "Elsewhere" / "不在 tmux"), separate from the tmux-based needs-you / working / idle groups, so users can see these sessions exist and their rough info (agent, project, state, idle time) without implying they can be jumped to or replied to.

#### Scenario: Native section rendered when native sessions exist
- **WHEN** the app polls `agents --json` and native sessions are present
- **THEN** they SHALL appear in a dedicated, clearly-labelled category distinct from the tmux groups

#### Scenario: Native rows expose no jump or reply affordance
- **WHEN** a native row is rendered
- **THEN** it SHALL NOT show a jump chevron or a reply/send control, and clicking it SHALL NOT attempt a terminal focus

### Requirement: Adopt-into-tmux action in the menu bar
The menu bar SHALL provide an action to adopt one or more selected native sessions into tmux, which resumes each conversation in a fresh tmux session/window. The action SHALL be available only for native rows whose agent is resumable and whose `session_id` is known, and SHALL surface the duplicate-instance warning before acting.

#### Scenario: Adopt a single native session
- **WHEN** the user triggers Adopt on an eligible native row
- **THEN** the app SHALL invoke the resume/spawn path to open a tmux session running that conversation, after showing the "close the original terminal" warning

#### Scenario: Adopt multiple native sessions
- **WHEN** the user selects several eligible native rows and triggers Adopt
- **THEN** the app SHALL resume each into its own tmux window/session

#### Scenario: Adopt hidden for ineligible sessions
- **WHEN** a native row's agent is not resumable or its `session_id` is unknown
- **THEN** the Adopt action SHALL NOT be offered for that row
