## ADDED Requirements

### Requirement: Clear default names for new dedicated HQ sessions

A newly created dedicated HQ tmux session SHALL be named `Gtmux HQ` when the name
is available, retaining the existing collision fallback. Its managed window SHALL
use the same readable HQ identity plus pane id. Existing sessions and manually
named adopted windows SHALL not be renamed. Detection SHALL continue to use the
HQ stamp and radar role precedence rather than this cosmetic name.

#### Scenario: Fresh launch versus an existing custom session

- **WHEN** gtmux creates a dedicated HQ session with no name collision
- **THEN** the session is named Gtmux HQ; focusing or adopting a custom-named
  session does not replace that session name
