## Tasks
- [x] Add Store package pre-upload verification and meaningful negative tests.
- [x] Preserve archive/symbol evidence through existing encrypted recovery.
- [x] Split hosted assets into reusable build, checkpointed capture and rendering with provenance tests.
- [x] Add ASC processing progress/resume and independent readiness readback tests.
- [x] Synchronize bilingual runbooks and capability requirements; run relevant gates.

## Acceptance
Native hosted capture, Apple processing and signed Store upload require dedicated live verification; synthetic tests do not establish those results. This task does not submit or alter the prepared live candidate. Disk limits prohibit local native builds.

## Evidence
58 discovered Python tests, including Ruby helper and Fastlane-gate subprocess checks, five deliberate mutations rejected; actual retained build 26 passed, superseded build 25 rejected for entitlement drift. Local make check, workflow lint and e2e TypeScript checks passed; design/spec gate is rerun after archive. Hosted native capture/recovery and live ASC placement are not yet accepted.
