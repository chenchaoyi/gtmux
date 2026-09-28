## ADDED Requirements

### Requirement: Ownerless Codex approval events do not trust an inherited pane

A Codex approval hook without cwd SHALL NOT use `TMUX_PANE` alone to set a
waiting marker. A unique live pane bound to its session ID MAY own the event.
When that binding is absent or ambiguous, the event SHALL remain pane-less and
SHALL NOT change another pane's state. The radar SHALL sense an active approval
menu in the pane that actually displays it.

#### Scenario: Shared app-server reports an HQ approval with a website pane environment

- **WHEN** the inherited pane is an idle website Codex pane and an HQ Codex pane displays a choice menu, but the hook has no session ID
- **THEN** the hook remains pane-less, the website pane remains idle, and radar reports the HQ pane as waiting from its live menu

#### Scenario: No unique approval pane

- **WHEN** a hook has no unique session binding
- **THEN** the event is journaled without a pane and the hook writes no pane waiting marker
