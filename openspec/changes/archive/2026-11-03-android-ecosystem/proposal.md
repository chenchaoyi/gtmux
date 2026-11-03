# Android ecosystem

## Why
The repository has a React Native Android template but no production identity/signing separation, paired deep links, document adapter, Android notification transport, native widget or Android acceptance/release pipeline. Android requires emulator coverage, repeatable automation, signed delivery and an accurate store-readiness checklist.

## What changes
- Keep one React Native product: reuse session/HQ/Chat/Terminal/Trace/Diff/readers/composer/profile/theme/layout and existing Go access rules.
- Configure real Android identity, canonical brand assets, version/build provenance, release signing, deep links, IME/insets and native platform adapters.
- Implement document/HTML-image preview, HQ-copy export, haptics/keyboard/text-selection and notification/widget boundaries without duplicating authorization or business models.
- Add isolated phone/tablet emulator automation, unsigned/QA CI, package/signature/16KiB checks, protected signed APK/AAB delivery and reproducible recovery.
- Extend the existing scoped distribution service for Android where needed, preserving its iOS behavior, privacy, recipients and regional policy. Prepare store prerequisites; missing legal/account actions are explicit, not faked.

## Surfaces
- terminal (CLI): the Go core remains cgo-free and authoritative for sessions, files, guest scopes and send authorization. FCM provider verdicts retire only explicitly revoked Android tokens.
- Menu bar (menubar): existing macOS traffic, focus, pairing and HQ behavior remain unchanged; no real panes are retargeted.
- phone (iPhone and Android): Android uses the same RN compact shell and contracts as iPhone, with native document, share, keyboard, widget and push adapters. iPhone compatibility has its existing regression gate.
- iPad: the shared regular shell also serves Android tablets; window resizing, large text and keyboard navigation keep one layout model. Native iPad adapters remain unchanged except for generated shared reader CSS.
- Web: Trace/code/API semantics remain shared. The private installation service adds typed APK delivery while preserving existing iOS links and access policy.

Provider and distribution credentials remain outside the repository and client packages. Public PR/CI fixtures are synthetic; store/account status and actual device acceptance are reported separately.
