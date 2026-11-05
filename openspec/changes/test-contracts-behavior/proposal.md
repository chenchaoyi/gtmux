# Strengthen client contracts and test execution

## Why
Independent HTTP fakes can drift from Go. Source guards alone do not prove mounted interactions. Sequential Android failures obscure their first cause, and fresh browser launches per case add avoidable overhead.

## What Changes
- Deterministic real-handler contracts checked independently by RN and Web fakes.
- Mounted draft, subscription and HQ human-input workflow regressions.
- Standalone Android capability blocks, explicit dependencies and timing.
- Measured browser process reuse with per-case context/server isolation; conservative proven test-only scopes.

## Surfaces
CLI/Go API, browser, RN iPhone/iPad/Android, test tooling and CI. No product permission, real-session writes, release, installation or Store changes.
