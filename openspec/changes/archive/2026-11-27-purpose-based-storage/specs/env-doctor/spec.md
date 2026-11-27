## ADDED Requirements

### Requirement: Recognize layout state before offering a move

Doctor SHALL distinguish a fresh root, a verified older layout, an interrupted move, conflicting copies, a completed move with recognized residue and a clean completed layout. Read-only inspection SHALL leave storage untouched. Repair SHALL preserve both copies on conflict and SHALL verify object identity before cleanup.

#### Scenario: Completed migration has an empty service recovery scaffold
- **WHEN** service restoration completed, the recorded layout is valid and the temporary service directories are empty
- **THEN** Doctor can remove the verified empty scaffold without moving data or restarting services

#### Scenario: Notification artwork is shared
- **WHEN** a new notification identifies an agent with a shipped icon
- **THEN** its thumbnail uses the canonical agent cache and subsequent notifications reuse the same unchanged file

### Requirement: Preserve data and references across offline reorganization

New installations SHALL organize managed data into sessions, HQ, attachments, services, diagnostics, backups and cache. Doctor SHALL provide the same organization for existing data after confirmation during offline maintenance. A client SHALL verify the active layout before selecting a write destination. An incomplete move SHALL keep ordinary writers fenced until the layout can be verified and parked service profiles have been returned.

#### Scenario: Historical attachment and new input after migration
- **WHEN** a user opens an authorized historical attachment link and sends input after migration
- **THEN** the link resolves to the original file and the updated client uses the current terminal input queue, preserving its receipts and deduplication records

#### Scenario: Roll back after new data was created
- **WHEN** Doctor returns a verified layout to the older format in an offline window
- **THEN** the reverse move includes registered data created since migration and stops before replacing an occupied destination
