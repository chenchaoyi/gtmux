# agent-dispatch (delta)

## ADDED Requirements

### Requirement: A persistent authentication notice does not fail the spawn ready gate forever

The spawn readiness gate (`prompt.IsComposerReady` / the two-frame settle) SHALL treat a
pane as ready when its composer prompt is present and the frame has settled, EVEN IF the
pane also carries a persistent `N MCP servers need authentication` / `run /mcp` notice. Such
a notice on an otherwise-ready, settled composer is standing chrome, NOT still-booting
noise, and SHALL NOT keep the pane not-ready indefinitely (which times out every spawn into
a false `NOT delivered`). A genuinely still-booting pane — a boot banner shown where the
composer will be, with no composer prompt yet — SHALL still be caught as not-ready.

#### Scenario: A ready composer with a permanent auth banner is ready

- **WHEN** a pane shows its composer prompt, has settled across two frames, and also shows a
  persistent `10 MCP servers need authentication · run /mcp` line
- **THEN** the ready gate reports ready and the goal is delivered

#### Scenario: A still-booting pane is still caught

- **WHEN** a pane shows a boot banner where the composer will be and no composer prompt yet
- **THEN** the ready gate reports not-ready

### Requirement: A ready-gate timeout names the line that blocked it

When the spawn ready gate times out, the failure evidence SHALL include the specific
boot-banner line that kept the pane not-ready, so a persistent-banner block is
distinguishable from a slow agent.

#### Scenario: The timeout evidence identifies the blocker

- **WHEN** the ready gate times out because a bottom line matched a boot-banner signature
- **THEN** the reported evidence contains that matched line, not only "composer not ready
  within the ready timeout"
