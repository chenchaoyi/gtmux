# Strengthen client contracts and test execution

## Why
Independent HTTP fakes can drift from Go. Source guards alone do not prove mounted interactions. Sequential Android failures obscure their first cause, and fresh browser launches per case add avoidable overhead.

## What Changes
- Deterministic real-handler contracts checked independently by RN and Web fakes.
- Mounted draft, subscription and HQ human-input workflow regressions.
- Standalone Android capability blocks, explicit dependencies and timing.
- Measured browser process reuse with per-case context/server isolation; conservative proven test-only scopes.

## Surfaces
- CLI / terminal: real Go authenticated routes on synthetic stores; no live pane input.
- menubar: existing native qualification stays required when selected; no runtime UI change.
- Web: independent fake contracts, real DOM and fresh contexts with shared browser processes.
- iPhone / phone: mounted RN draft/HQ/subscription interactions, not physical-device acceptance.
- iPad: the same RN policy; native/touch/VoiceOver acceptance stays separate.
- Android: complete phone/tablet QA journey plus isolated capability starts and failure timing.
- Test tooling / CI: conservative impact selection and reviewed case records.
No product permission, real-session writes, release, installation or Store changes.
