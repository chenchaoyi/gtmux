## ADDED Requirements

### Requirement: Paired device labels can be edited on the Mac

The Mac preferences SHALL offer editing a paired-device display name, explicitly described as a roster label rather than a system device name. A master-authenticated POST /api/devices/rename SHALL accept a stable device ID and a nonblank UTF-8 name bounded to 40 Unicode characters, persist it using the roster's ordered write path, and preserve tokens, scope, platform and connection metadata. Guest entries SHALL NOT be renamed through this endpoint. Owner-device and guest tokens SHALL be refused. The editor SHALL retain input and show failure when the request fails, closing and refreshing only after success. Existing clients SHALL continue to read the name field. The additive nameIsCustom marker SHALL preserve literal custom labels instead of applying legacy display cleanup.

#### Scenario: Two generic iPhones

- **WHEN** the owner edits one paired iPhone's name on the Mac
- **THEN** its stored label changes on roster-reading surfaces without re-pairing or changing its token

#### Scenario: A paired phone attempts device administration

- **WHEN** an owner-device token calls the rename endpoint
- **THEN** it receives 403 and the roster remains unchanged

#### Scenario: Client details become available

- **WHEN** a browser enrolls or reconnects using its authenticated token
- **THEN** available browser family, major version, OS and address are recorded for the roster
- **AND** unknown details remain explicitly unknown rather than guessed
