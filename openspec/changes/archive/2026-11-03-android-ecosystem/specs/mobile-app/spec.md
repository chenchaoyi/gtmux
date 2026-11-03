## ADDED Requirements

### Requirement: Android shares the mobile product
Android SHALL reuse the React Native session/HQ/Chat/Terminal/Trace/Diff/composer and access models, with native adapters for operating-system capabilities. It SHALL preserve paired owner/guest scopes, actual turn numbers, bounded history and cross-client labels/icons. Release artifacts SHALL use an explicit Android identity, source/version provenance and a private release signer, never the public QA signer.

#### Scenario: Android opens an existing session
- **WHEN** a paired owner opens a session on an Android phone or tablet
- **THEN** the existing shared views display the same session, original turns, tools and capabilities, with safe insets and usable Back/IME behavior

### Requirement: Android acceptance and delivery are explicit
Android delivery SHALL verify APK/AAB identity, version, signature, native library alignment and source provenance. Isolated emulator interactions, unit checks, screenshots, physical-device behavior, live push and Store publication SHALL be reported separately. Missing account/provider setup SHALL not become a false success or silent no-op.

#### Scenario: Push provider is not configured
- **WHEN** Firebase or its server provider is unavailable
- **THEN** the app remains usable, identifies unavailable background push and does not register an FCM token as APNs or claim tested delivery
