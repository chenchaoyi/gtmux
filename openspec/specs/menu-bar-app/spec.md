# Menu-Bar App Specification

## Purpose

An always-visible macOS menu-bar app that shows, at a glance, the most-urgent
agent state and a popover list grouped by who needs you. It is a pure consumer of
the CLI (polls `gtmux agents --json`, shells out to `gtmux focus`) and the click
target for notifications.
## Requirements
### Requirement: Ambient status item

The system SHALL render an `NSStatusItem` whose glyph encodes the most-urgent
state by COLOR — the brand-grid mark tinted to the state's palette color
(waiting → working → idle → calm) — with a count badge of the most-urgent
actionable count. (Since the 2026-06 UI overhaul, #160, the glyph is color-only:
one tinted brand mark for every state, NOT a per-state shape.)

#### Scenario: Most-urgent wins

- **WHEN** at least one agent is waiting
- **THEN** the status item's brand mark is tinted the waiting (red) color and shows
  the waiting count badge

### Requirement: Grouped popover

The system SHALL show a popover listing agents grouped in fixed order
waiting → working → idle → running, only non-empty sections, each row carrying
the agent avatar + status badge + session/task, with the waiting section
emphasized.

#### Scenario: Jump from a row

- **WHEN** a row is clicked (or Enter / ⌘1–9)
- **THEN** the app runs `gtmux focus <pane>` and lands on that agent

### Requirement: Pure CLI consumer

The system SHALL source all data from `gtmux agents --json` and SHALL NOT
duplicate detection logic; gtmux-core stays the single data source.

#### Scenario: Poll for updates

- **WHEN** the refresh timer fires
- **THEN** the app re-runs `gtmux agents --json` and repaints

### Requirement: Notification click target

The system SHALL be the notification target (`com.gtmux.menubar`): it drains the
notify queue, posts native banners, and on click jumps to the last-finished
agent.

#### Scenario: Click a banner

- **WHEN** the user clicks a delivered notification
- **THEN** the app activates and runs `gtmux focus --last`

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

### Requirement: Mark errored idle rows in the popover

The menu-bar popover SHALL visually distinguish an idle agent that ended on an
error (`error: true` in the `agents --json` contract) from a successfully-finished
idle agent, using an amber ⚠ "errored" modifier and the `error_text` summary in
place of the green ✓. The row SHALL remain in the IDLE section and MUST NOT use the
red `waiting`/needs-you color.

#### Scenario: Errored idle agent

- **WHEN** an agent row has `status: idle` and `error: true`
- **THEN** the popover renders it in the IDLE section with an amber ⚠ marker (not
  the green ✓) and shows the `error_text` summary
- **AND** it is not colored red and does not sort into NEEDS YOU

#### Scenario: Successful idle agent unchanged

- **WHEN** an agent row has `status: idle` without `error`
- **THEN** the popover renders it exactly as today (green ✓)

