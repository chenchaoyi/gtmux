- [x] Review and seal observable read/error/empty invariants before test changes
- [x] Implement scoped bounded reads with preserved trailing refresh and legacy guards
- [x] Implement shared loading/genuine-empty/error presentation
- [x] Verify production counterexamples and preserve authorization assertions
- [x] Update bilingual design/current spec and complete required gates

## Acceptance

The complete App check passed with 2,507 tests and 15 existing skips. Production counterexamples cover the premature zero count, the 600 ms pull indicator and malformed successful response bodies; authorization, guest revocation and Live Activity regressions remain checked. Native visual verification, iPad, Dynamic Type and VoiceOver were not performed for this change. No real device, server data or notification registration was used.
