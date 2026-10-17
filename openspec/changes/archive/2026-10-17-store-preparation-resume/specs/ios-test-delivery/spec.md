## ADDED Requirements

### Requirement: Store package verification precedes binary upload

The canonical Store lane SHALL preserve its archive and require matching app/extension symbols before uploading. Existing-IPA uploads SHALL require an original IPA or archive baseline and explicit version/build. Both paths SHALL verify all three signatures, Store profiles, exact identity/version, executable/resource continuity and signed DER entitlement continuity, permitting the Store beta-reporting difference and, only for development-signed archive baselines, disabling debugger access and switching enabled APNs to production. Keychain/app groups and all other rights SHALL remain exact. Failure SHALL prevent upload. An IPA fallback without symbols SHALL retain that limitation. Encrypted hosted recovery SHALL retain sanitized archive Products, Info and symbols, exclude profiles/signing setup/external links and document expiry.

#### Scenario: Resigning adds a generic keychain access group
- **WHEN** signed permissions differ from the original beyond beta-reporting
- **THEN** verification fails before any binary upload

#### Scenario: Archive symbols do not match an extension
- **WHEN** the canonical archive lacks matching native UUID evidence
- **THEN** the canonical Store lane cannot upload

### Requirement: Hosted Store assets support validated reuse by stage

Manual main-only capture SHALL separate unsigned simulator builds, checkpointed device/locale captures and rendering. Reused runs SHALL belong to the same repository/workflow, be manual on main and completed. Build source/toolchain/architecture/content and recorded group hashes SHALL be verified before reuse. Changed capture inputs SHALL invalidate group completion. Corruption SHALL fail closed. Each completed locale SHALL be preserved even if a later one fails. Rendering-only SHALL require all four groups for the exact release and record current renderer/caption identity separately from original app/capture evidence. No signing credentials or real data SHALL enter public artifacts.

#### Scenario: The second locale fails after the first completed
- **WHEN** the operator resumes using the completed failed run
- **THEN** the verified build and first locale may be reused without recapture
- **AND** incomplete or mismatched groups must run again

#### Scenario: Only captions change
- **WHEN** rendering-only selects a complete matching capture checkpoint
- **THEN** no native build or simulator is needed
- **AND** the manifest preserves capture origin and records the new rendering inputs

### Requirement: Store processing and readiness are explicit and independently checked

ASC preparation SHALL flush progress, optionally retain an owner-only journal without credentials/URLs, and bound image processing waits. Pending processing SHALL retain a resumable result without duplicate uploads or premature placement replacement. Independent status SHALL compare the actual IPA receipt, VALID selected build, expected localized version text, content-matched screenshot groups and active creative placements. Errors SHALL invalidate earlier readiness. Only all passing checks SHALL report ready for user review. Account forms, review contact and visual preview SHALL remain separately documented checks; final Submit SHALL remain a user action.

#### Scenario: Apple processing outlasts the configured wait
- **WHEN** an uploaded image remains processing at the deadline
- **THEN** the command exits with a pending status and reruns reuse that image
- **AND** existing group placements remain until replacement images are ready

#### Scenario: One readiness check or API call fails
- **WHEN** text, build, package or asset evidence is incomplete or unavailable
- **THEN** the current saved status is not ready
- **AND** preparation never submits the version
