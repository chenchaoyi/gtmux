## ADDED Requirements

### Requirement: Recognize layout state before offering a move

Doctor SHALL distinguish a fresh root, a verified older layout, an interrupted move, conflicting copies, a completed move with recognized residue and a clean completed layout. Read-only inspection SHALL leave storage untouched. Repair SHALL preserve both copies on conflict and SHALL verify object identity before cleanup.

#### Scenario: Completed migration has an empty service recovery scaffold
- **WHEN** service restoration completed, the recorded layout is valid and the temporary service directories are empty
- **THEN** Doctor can remove the verified empty scaffold without moving data or restarting services

#### Scenario: Notification artwork is shared
- **WHEN** a new notification identifies an agent with a shipped icon
- **THEN** its thumbnail uses the canonical agent cache and subsequent notifications reuse the same unchanged file
