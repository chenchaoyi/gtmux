## ADDED Requirements

### Requirement: Mobile shows native sessions in an "Elsewhere" section
The mobile app SHALL group `source: "native"` sessions into their own "Elsewhere / 不在 tmux" section, separate from the tmux status groups. These rows are sense-only: they carry a `native` tag, no jump chevron, and tapping one SHALL NOT open a terminal mirror (there is none). Moving a native session into tmux stays a menu-bar/CLI action; the mobile app is display-only for the native category.

#### Scenario: Native section on mobile
- **WHEN** the phone polls the radar and native sessions are present
- **THEN** they SHALL appear in a dedicated "Elsewhere" section, marked non-tappable (no terminal), distinct from the tmux groups

#### Scenario: Tapping a native row does nothing
- **WHEN** the user taps a native row on mobile
- **THEN** the app SHALL NOT navigate to a terminal/detail view for it
