## Implementation
- [x] Add an owner-only bounded workspace read adapter and contract tests.
- [x] Add the adaptive mobile overview, matter reader and old-server fallback.
- [x] Cover stale reads, authorization, empty/error states and exact participant identity.
- [x] Capture current-source phone/iPad light/dark views with synthetic data.
- [x] Update specs and bilingual user documentation.

## Acceptance
Simulator and synthetic fixture evidence is separate from physical-device acceptance. This iteration does not publish or install a new production release.

Current-source iPhone/iPad light/dark overview and matter sheets were rendered in an independent simulator fixture. Native taps and VoiceOver remain unverified because the tool returned only the App root. React component actions, stale/auth/old-server boundaries and existing HQ input tests are recorded separately.
