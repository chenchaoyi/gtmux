## ADDED Requirements

### Requirement: Conversation readers keep earlier windows reachable
Browser Chat SHALL expose existing bounded earlier-history cursors in focus and
tiles, reuse shared transcript merge rules, and retain the visible reading anchor,
expanded tools and code selection through prepends and recent-window refreshes.
It SHALL fence requests by current credential and canonical conversation identity.

#### Scenario: Earlier history arrives while recent output updates
- **WHEN** the reader loads an earlier page and a recent-window poll completes
- **THEN** contiguous history remains ordered, the visible turn stays anchored,
  and missing overlap or source reset is explicit rather than silently joining a gap

#### Scenario: A request loses scope or its target changes
- **WHEN** history is denied or the active conversation changes before completion
- **THEN** the stale reply cannot populate another conversation or grant input

### Requirement: Readable external sessions share desktop tile management
Readable external conversations SHALL support desktop double-click and drag to
independent, resizable workbench tiles. Saved layouts SHALL resolve their canonical
session/home identity against the current authorized roster. Presentation SHALL NOT
create a tmux pane, terminal input permission or a replayable execution command.

#### Scenario: Two external conversations are opened together
- **WHEN** the reader opens two readable external sessions on a desktop
- **THEN** both tiles coexist, retain their own conversation and geometry, and can
  enter and leave focus without replacing the other tile

### Requirement: Reader chrome follows the App vocabulary
Turn rows SHALL avoid repeating the agent name on every turn. Expanded Live SHALL
use a readable terminal surface and retain manual scrolling. Code actions SHALL
have consistent icon-button chrome and accessible names. Horizontal indicators
SHALL have reading space clear of the final text baseline.

#### Scenario: Code and execution paths scroll horizontally
- **WHEN** the reader reaches the bottom and scrolls a long line horizontally
- **THEN** the last line stays readable and its copy action remains reachable
