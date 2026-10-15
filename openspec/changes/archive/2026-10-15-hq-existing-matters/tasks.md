## Implementation
- [x] Add idempotent registration with typed provenance and no delivery side effects.
- [x] Teach enrollment across instruction channels and bump the managed charter.
- [x] Add the empty-state board/review paths and provenance detail, preserving drafts.
- [x] Cover empty homes, retry/conflict, failed storage, non-HQ calls and draft behavior.
- [x] Synchronize bilingual docs/spec and run targeted/core/design checks.

## Acceptance
Use synthetic homes and fake clients. No migration of personal HQ records. Native runtime
layout/VoiceOver must be reported separately; disk is red, so avoid a fresh local Swift build.

Local verification: complete related Go packages with race, make check, source design
conformance and Swift frontend syntax parse passed. Two deliberate regressions (queueing
a registration for delivery, resetting state on retry) failed the new tests, then were
restored. Swift build/tests run in hosted CI; real layout and VoiceOver are not claimed.
