## Implementation
- [x] Add bounded access-history model/reader and request/lifecycle recording with verified identities, redaction and read coalescing.
- [x] Add owner-only API and CLI history; validate pagination, authorization, failures and retention boundaries.
- [x] Add native and phone/iPad viewers with loading, errors, empty history and earlier-page states.
- [x] Harden append permissions and doctor inspection with isolated permission/symlink tests.
- [x] Synchronize specs, API contract and bilingual docs; run core/design/platform gates and archive the change.

## Acceptance
No release/install or live configuration changes. Native and device VoiceOver acceptance require a separate observed result; hosted builds and component tests are not equivalent.

Hosted CI and device acceptance are reported separately. Local core/design and mobile checks passed; real-device keyboard, layout and VoiceOver remain unverified.
