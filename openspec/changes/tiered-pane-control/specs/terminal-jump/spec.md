# terminal-jump — delta

## ADDED Requirements

### Requirement: Jump and type target any pane regardless of agent

`gtmux focus <pane-id>` and `gtmux send <pane-id>` SHALL operate on any live tmux
pane, whether or not it runs a coding agent — the jump/type primitives are pane-level
and do not require the target to be on the agent radar. When the target pane exists
but is not (or is no longer) running an agent, `gtmux focus` SHALL still jump to it
(its content is often exactly what the user wants to see) and SHALL make clear that
the pane is a plain shell rather than silently landing the user on it as if it were
still an agent. When the target pane no longer exists at all, `gtmux focus` SHALL
report that instead of jumping.

#### Scenario: Focus a pane whose agent has exited

- **WHEN** `gtmux focus %N` targets a pane that exists but whose coding agent has since
  exited (it is now a plain shell)
- **THEN** focus jumps to the pane and states that the agent there has exited and it is
  now a plain shell

#### Scenario: Focus a plain pane that never ran an agent

- **WHEN** `gtmux focus %N` targets a live plain pane that is not on the agent radar
- **THEN** focus jumps to it normally, treating a non-agent pane as a first-class jump
  target
