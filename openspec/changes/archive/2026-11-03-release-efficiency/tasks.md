## Implementation

- [x] Add explicit version stamping and platform notes preflight with isolated Git regressions.
- [x] Build Android QA once and require independent phone/tablet checks of the same package.
- [x] Add shared encrypted-artifact retrieval with transport, identity, cache and archive-failure tests.
- [x] Update maintainer instructions and capability specs; perform code/security review and required checks.

## Acceptance

- Confirm CI runs both full emulator suites against identical APK bytes and fails on either missing/failed leg.
- Exercise read-only retrieval against an existing source-pinned delivery; no new package publication.
- Report cloud queue time separately; no promise of recovered runner capacity or physical-device acceptance.
