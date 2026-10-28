## ADDED Requirements

### Requirement: Consistent native pairing delivery
Both native pairing entry points SHALL use shared components for the owner access explanation, measured reachability, the app QR with scan guidance, full selectable browser link and terminal command, and code renewal countdown and refresh. Both SHALL use one-time enrollment codes from PairStore, preserve route controls and existing renewal behavior, and SHALL NOT add a terminal command to guest share delivery.

#### Scenario: Pair from either native entry
- **WHEN** the user opens the standalone pairing window or the pairing sheet
- **THEN** the same pairing code can be used through the app, browser or terminal, with the same explanatory and renewal controls
