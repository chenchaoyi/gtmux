## ADDED Requirements

### Requirement: Legible and coherent mobile launch identity
The mobile launch presentation SHALL retain the gtmux pane-grid identity and cyan accent with neutral panes visible in both light and dark appearance. The static native launch and RN startup view SHALL share the same mark and theme background. RN supporting copy and loading feedback SHALL remain readable without covering the safe area or requiring an artificial minimum launch duration. Loading motion SHALL respect Reduce Motion. Phone and iPad layouts SHALL use the same restrained identity without stretching the mark.

#### Scenario: Light appearance
- **WHEN** the app opens with a light appearance
- **THEN** all neutral logo panes remain distinct from their ground
- **AND** supporting copy has readable contrast

#### Scenario: Native-to-RN handoff
- **WHEN** native launch hands off to the RN startup view
- **THEN** the mark and background remain visually coherent
- **AND** readiness immediately exits the startup view without an imposed delay

#### Scenario: Adaptive launch layout
- **WHEN** the app uses an iPad, landscape, larger text or Reduce Motion
- **THEN** the mark retains its proportions and supporting/loading content stays reachable
- **AND** reduced-motion loading remains static
