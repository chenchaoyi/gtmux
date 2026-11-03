## ADDED Requirements

### Requirement: Android emulator diagnostics survive driver failure

The owned-emulator runner SHALL retain a bounded device-log tail independently of WebDriver screenshots and source capture. A lost automation connection SHALL preserve its first failed flow, mark dependent later flows as not run, and keep a nonzero exit. Diagnostic capture SHALL NOT restart shared ADB, reuse a physical device, or silently turn a partial run into success.

#### Scenario: Instrumentation stops while reading Trace

- **WHEN** the owned emulator loses its automation connection during a flow
- **THEN** screenshot/source failure does not prevent independent device-log retention
- **AND** later flows are recorded as not run and the required CI check fails
