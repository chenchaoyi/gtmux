## ADDED Requirements

### Requirement: Owner revision-checked desktop follow
The session-follow endpoint SHALL permit only owner credentials and verified desktop session IDs. GET SHALL return current permissions and revision. POST SHALL compare revision, ignore caller timestamps, persist atomically and return an honest failure for conflict or I/O errors.

#### Scenario: Guest
- **WHEN** a valid guest token requests GET or POST
- **THEN** the request is refused before accessing the policy

#### Scenario: Conflict
- **WHEN** POST carries an outdated revision
- **THEN** 409 is returned and stored settings remain

#### Scenario: Unsupported identity
- **WHEN** an owner requests a terminal or unknown conversation
- **THEN** 422 is returned and no policy is written
