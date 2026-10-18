## ADDED Requirements

### Requirement: App Store readiness includes a version-bound accessibility review

The Store preparation workflow SHALL require a private review bound to the exact
candidate version, build and package-receipt hash. It SHALL compare the current live
version/build and published declarations with the review baseline and require separate
iPhone/iPad common-task evidence, all capability decisions, limitations, change impact
and optional Accessibility URL review. Missing, incomplete, mismatched or stale reviews
SHALL prevent ready_for_user_review and SHALL be checked before asset-placement writes.
Rechecking SHALL replace stale ready results, including on API failure.

#### Scenario: A previous review is reused for another candidate
- **WHEN** the package receipt, version or build differs from the reviewed candidate
- **THEN** preparation reports incomplete with an actionable reason
- **AND** it does not place assets or treat the previous receipt as current approval

#### Scenario: New support exists only in the candidate
- **WHEN** a reviewed capability is absent from the live declarations
- **THEN** readiness requires an explicit post-live owner and follow-up plan
- **AND** neither preparation nor review commands publish the future claim

#### Scenario: Support regresses or the live baseline changes
- **WHEN** a candidate removes an existing claim or the current live baseline differs
- **THEN** preparation requires a fresh review and resolution before readiness

### Requirement: Accessibility publication is verified after the app goes live

A read-only post-live check SHALL compare the exact live version/build and both
device-family published declarations against the reviewed candidate decisions.
Pending app release, missing declarations or mismatched claims SHALL remain incomplete.
The workflow SHALL retain private evidence and require operator publication/preview;
it SHALL NOT infer VoiceOver acceptance from labels, CI or simulator screenshots.
The optional Accessibility URL SHALL NOT become mandatory. App Submit SHALL remain
a separate operator action.

#### Scenario: A build is approved but not yet live
- **WHEN** the reviewed candidate is not the live version/build
- **THEN** the post-live check remains incomplete and does not publish declarations

#### Scenario: The live version and claims match
- **WHEN** the exact candidate is live and both families' published claims match
- **THEN** the read-only check records alignment without submitting or changing metadata
