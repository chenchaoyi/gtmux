## MODIFIED Requirements

### Requirement: The empty panel offers the two ways out of it

With no agents on the radar, the popover SHALL offer a concise explanation and
a primary New session row, describing that it opens a tmux terminal where the
user starts their agent. The existing restore row SHALL remain available when
a working set exists. It SHALL NOT repeat the app mark from the header.

Manual terminal instructions SHALL be collapsed by default. Expansion SHALL
show an agent-neutral tmux command with Copy and brief confirmation, followed
by a separate step to start an agent. Examples SHALL include Codex and Claude
and name other supported agents, without implying Claude-only support.

#### Scenario: An empty radar is actionable

- **WHEN** the radar is empty
- **THEN** New session is visible without expanding manual instructions

#### Scenario: Manual startup is agent-neutral

- **WHEN** the user expands the manual instructions
- **THEN** Copy copies the tmux-only command, and a separate step explains
  starting the user's chosen agent
