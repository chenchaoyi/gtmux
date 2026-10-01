## ADDED Requirements

### Requirement: Verified HQ identity in pane browsers

The pane producer SHALL carry the radar's supervisor role on the verified HQ agent
pane as an optional `role` field. It SHALL not classify HQ independently from names
or directory paths, or give native/watched plain panes that role. Guest filtering
SHALL continue to apply per pane before rows reach a client.

Phone, iPad, menubar and Web pane browsers SHALL mark the containing session header
and the supervisor pane with a neutral HQ badge. Older-core rows MAY use the radar
join by pane id. A confirmed HQ's legacy default session name `HQ` or `hq` SHALL
be displayed as `Gtmux HQ`; custom names SHALL be retained. Grouping, collapse,
focus and locators SHALL retain raw names/ids. The display name SHALL be searchable,
and filtering a sibling pane SHALL not remove the session's verified HQ marker.
The terminal pane tree SHALL mark the verified supervisor without changing locators.

#### Scenario: Legacy HQ and an ordinary same-named session

- **WHEN** one agent has the verified supervisor role and another session is merely named HQ
- **THEN** only the verified HQ is marked; its legacy name displays as Gtmux HQ

#### Scenario: Search and fold a mixed session

- **WHEN** a verified HQ shares a session with a plain shell
- **THEN** searching the displayed HQ name finds the supervisor, searching the shell
  retains the group's HQ marker, and folding uses the original session key
