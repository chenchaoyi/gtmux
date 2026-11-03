## ADDED Requirements

### Requirement: Parallel Android checks preserve one build identity

The selected Android CI SHALL build and lint the QA APK once, record its source and SHA256, and run the complete phone and tablet emulator suites in independent jobs using those identical bytes. Artifact names SHALL include run attempt. Missing artifacts, source mismatch, checksum failure, skipped or failed selected legs SHALL fail the aggregate. Each leg SHALL retain separate evidence.

#### Scenario: One emulator leg fails

- **WHEN** the phone passes but the tablet fails or is cancelled
- **THEN** the required aggregate fails, while phone evidence remains available

### Requirement: Early release preparation and bounded retrieval

Maintainers SHALL be able to stamp an explicit stable version without creating a tag and preflight version and bilingual notes for every selected mobile platform before opening the release PR. Android notes SHALL remain independent of Apple notes. Retrieval SHALL require an exact successful source/run/attempt/workflow and a nonexpired encrypted artifact with a published digest. GET retries SHALL be bounded, credentials SHALL NOT cross an origin boundary, and corrupted or partial downloads SHALL NOT be exposed as verified results.

#### Scenario: Android notes are missing

- **WHEN** an Android release is selected without that version's bilingual Android entry
- **THEN** the read-only preflight fails before a native build or tag publication

#### Scenario: The transport fails transiently

- **WHEN** an artifact GET fails before completion
- **THEN** retrieval may retry within its bound, and only complete digest-verified encrypted bytes become output
