## Implementation
- [x] Add native extension and reuse shared views/models.
- [x] Add bounded parent publisher, configuration awareness and usage handoff.
- [x] Build metadata in the thin Xcode host and extension; sign with a derived group and both architectures.
- [x] Update platform specs and paired documentation.
## Verification
- [x] Run model, privacy, rendering and actual bundle metadata checks, including a missing-host-metadata negative control.
- [x] Verify signed gallery rendering and real sandbox sharing with synthetic data; test strict usage-route parsing.
- [x] Run all 572 Mac tests in isolated single-worker processes and repository validation.

Validation boundaries: the signed fixture is not a notarized release or an installed
production version. No native Intel Mac, macOS 14 runtime, VoiceOver, real-account
background refresh or actual URL click-through has been validated. A user-authorized temporary desktop widget displayed the synthetic quota reading.
Provider editing remains unverified because the automation tool cannot reliably
target this widget among Notification Center's windows.
Same-process AppKit test timeouts and the unmodified-baseline reproduction are
retained; isolation passing does not identify their full underlying cause.
