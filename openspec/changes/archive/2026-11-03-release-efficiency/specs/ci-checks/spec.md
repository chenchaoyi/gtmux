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

### Requirement: Verify actual native target coverage

Unsigned iOS CI SHALL retain the normal standard host and explicitly compile for ARM64 Simulator. It SHALL inspect the actual app and both embedded extension executables before success. Missing products or an unexpected architecture SHALL fail the gate. The complete native build SHALL pass on the actual candidate and main, without signing credentials or changes to the production delivery runner.

#### Scenario: An extension target drifts

- **WHEN** an embedded extension is missing or compiled for an unexpected architecture
- **THEN** the unsigned native gate fails even if xcodebuild itself returned success

### Requirement: Check effective public commit identity

Release preflight SHALL inspect the effective author and committer identities before platform preparation succeeds. It SHALL accept the project public or GitHub no-reply identity, redact rejected configured addresses, and never change shared Git configuration. A separate identity-only mode SHALL be available before ordinary public task commits. This check SHALL NOT claim removal of metadata already published.

#### Scenario: A work identity is inherited

- **WHEN** the effective author or committer uses an address outside the public identity policy
- **THEN** preparation fails without printing that address or changing configuration
