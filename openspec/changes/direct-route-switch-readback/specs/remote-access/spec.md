# remote-access (delta)

## MODIFIED Requirements

### Requirement: A Mac can move between Direct servers without re-pairing its devices

After a move succeeds, the current server in the CLI, menu bar, and owner route list SHALL
be resolved from this Mac's persisted dial URL among the offered servers. A stale
provisioner `current` value SHALL NOT replace a known local route. When the local URL is
absent or not offered, the provisioner value MAY be used. The menu bar SHALL NOT refresh
the route list concurrently with a move.

#### Scenario: Stale list right after a move

- **WHEN** the Mac has saved the destination URL but the provisioner still lists the old
  server as current
- **THEN** the destination remains marked current after the menu bar reloads the list

#### Scenario: Refresh requested while moving

- **WHEN** a refresh is requested while a menu-bar route move is in progress
- **THEN** the refresh waits for the move's own post-completion reload
