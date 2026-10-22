## ADDED Requirements

### Requirement: Official releases have independently verified mirrors
Default release discovery and installation SHALL authenticate bounded RSA/SHA256 metadata with the embedded public key before trusting owned mirror bytes. The exact binary and installer source, stable version, complete permitted asset names, SHA256 and byte counts SHALL be recorded. Signature/schema failures SHALL stop without weaker fallback. HTTP/network unavailability MAY retain the original GitHub trust channel. Once verified metadata is present, fallback archives SHALL match it. Owned mirror requests SHALL not follow redirects. Explicit source overrides SHALL preserve their previous behavior. The native updater SHALL not execute a remote script.

Publication SHALL require exact-source official CLI and notarized Mac build success, unchanged GitHub API asset digests and CLI checksums, authenticated writes and immutable version objects. Both public routes SHALL be read back before atomic stable promotion; incomplete, modified, downgraded or concurrent publication SHALL not advance stable. Public downloads SHALL not expose the private phone bucket, credentials, listing or unintended default hostnames. Static website bootstraps SHALL verify and pin the signed installer; the initial HTTPS bootstrap trust SHALL remain explicit.

#### Scenario: A mirror supplies modified metadata
- **WHEN** a signed manifest fails authentication or scope validation
- **THEN** the updater and bootstrap stop without trusting an unsigned index

#### Scenario: A partial release is uploaded
- **WHEN** one component, checksum or public readback is missing or different
- **THEN** stable remains unchanged and no upload is described as a complete delivery

#### Scenario: Website and phone channels differ
- **WHEN** official Mac/CLI download hosting changes
- **THEN** website setup and download links reflect the deployed service
- **AND** App Store/private phone delivery remains independent, with no hosting-only phone build

## MODIFIED Requirements

### Requirement: Optional app failure preserves CLI installation facts

The local installer SHALL verify and replace a selected CLI before attempting the
optional app. It SHALL retain authoritative checksum verification, signed official-mirror or direct GitHub app
transport, archive validation and app rollback. App download, replacement or activation
failure SHALL preserve the CLI result and report both component outcomes. Exit 3 SHALL
mean the app step is incomplete; exit 1 SHALL mean a required installation step failed.
Release notes SHALL cover only components that were installed, including CLI success
before app failure. Only a replaced CLI SHALL trigger service refresh and the CLI's
post-update check. The menu bar SHALL present exit 3 as an incomplete update with retry,
retaining the component receipt even when release notes follow it.

#### Scenario: App download or replacement fails after CLI success

- **WHEN** the CLI is installed but the app cannot be downloaded or replaced
- **THEN** CLI bytes and their success receipt remain, notes remain available and exit is 3
- **AND** the app is not reported as installed

#### Scenario: App activation fails after replacement

- **WHEN** app replacement succeeds but activation fails
- **THEN** the receipt distinguishes installed files from failed activation and offers retry
