## ADDED Requirements

### Requirement: Application restore tolerates inherited terminal markers

Explicit application restore invocations SHALL not be rejected solely because the GUI inherited TMUX from its launcher. The selected socket environment and existing additive restore behavior SHALL be preserved. Ordinary terminal invocations from inside tmux SHALL remain refused.

#### Scenario: A menu-bar app was launched by tmux

- **WHEN** the app invokes restore with its explicit application flag and inherited TMUX
- **THEN** restore reaches the pending-session path without replacing live sessions or clearing the socket environment
