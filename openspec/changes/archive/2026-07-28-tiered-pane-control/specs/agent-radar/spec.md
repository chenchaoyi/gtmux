# agent-radar — delta

## ADDED Requirements

### Requirement: The radar admits only agents and opt-in watched panes

The agent radar SHALL list coding-agent panes automatically and SHALL NOT
automatically list any non-agent pane. The ONLY non-agent rows permitted on the radar
are panes a user has explicitly promoted via "watch this pane"; such a row SHALL be
marked as a WATCHED pane, distinct from agent rows, and SHALL NOT be assigned an agent
status (waiting/working/idle/running) — a watched plain pane has no agent turn to be
in those states. This keeps the radar's core signal ("which agent needs you") intact
while allowing a deliberate, clearly-labeled exception.

#### Scenario: A plain pane is never auto-added

- **WHEN** the fleet contains agent panes and unrelated plain shell panes, and no pane
  has been watched
- **THEN** the radar lists only the agent panes; no plain pane appears

#### Scenario: A watched pane is marked distinctly

- **WHEN** a user has watched a plain pane
- **THEN** it appears on the radar with a watched indicator and no agent status glyph,
  distinguishable at a glance from an agent row
