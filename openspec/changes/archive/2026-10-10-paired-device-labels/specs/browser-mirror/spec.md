## ADDED Requirements

### Requirement: Corresponding conversation controls use consistent semantics

The browser focus and tile views SHALL order Chat before Terminal for agents, matching the phone and iPad. Plain panes SHALL NOT offer Chat and opening them SHALL NOT replace the saved agent mode. The initial default SHALL remain Terminal. Plain-pane titles SHALL remain consistent between the roster and focus. Workbench-only Diff SHALL remain available. Visible status and accessible status names SHALL use the same mapping. All-panes fold controls SHALL use Collapse all and Expand all in English and 折叠全部 and 展开全部 in Chinese.

#### Scenario: Switch from a conversation to a shell

- **GIVEN** an agent uses the saved Chat mode
- **WHEN** the user opens a plain shell pane and then an agent
- **THEN** the shell exposes only Terminal and the agent restores Chat

#### Scenario: Corresponding agent tabs

- **WHEN** the user opens an agent in focus or a workbench tile
- **THEN** Chat appears before Terminal in either language
