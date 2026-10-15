## Tasks
- [x] Correct 304/cancellation/deadline recording and legacy diagnostic presentation.
- [x] Align pane controls and verify focus/toggle behavior in rendered component tests.
- [x] Add meaningful request/timeout/retained-cache regression coverage.
- [x] Update paired docs, run gates and sync/archive the spec.

## Acceptance
Source and rendered-component tests do not constitute physical iPhone/iPad layout or
VoiceOver speech acceptance. No large simulator/device build is required for this patch;
report those checks as unverified unless a real acceptance run is performed.

## Verification
- Mobile gate: TypeScript and ESLint (zero errors; existing warnings), 1,660 Jest tests passed, 15 existing skipped tests.
- Targeted six suites: 115 tests passed, covering both compact and regular pane controls, cache validation/recovery, cancellation and push/read deadlines.
- Mutation checks caught restored 304 warnings, incorrectly silenced timeouts and a disconnected search focus action; production code restored.
- Core `make check` and design conformance passed. The initial sandboxed mobile run was stopped after loopback `listen EPERM`; the complete rerun used an isolated synthetic server with loopback permission.
