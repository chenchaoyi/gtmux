# Tasks

- [x] Add scoped, cancellable mobile read tasks and migrate the affected polling paths.
- [x] Bound and coalesce Web polling while retaining response-order and credential fences.
- [x] Bound native periodic CLI reads and guard their concurrency without clearing valid data on failure.
- [x] Cover slow reads, deadline recovery, disposal, manual refreshes and real consumer wiring; verify mutations.
- [x] Run relevant platform checks, make check and design/spec validation; synchronize bilingual docs/specs and archive.

## Acceptance

Real-device layout and VoiceOver are not verified by the read-lifecycle tests.

Validation: full make check passed; mobile typecheck/lint and Jest passed (1,650 tests,
15 intentional real-server skips); all 434 Swift tests passed; 53 Web logic tests and
four rendered Chromium scenarios passed. Removing the mobile admission guard, Web
coalescing guard or actual AgentStore guard made the corresponding regression tests
fail; all source was restored before final gates. The first unprivileged full mobile
run was interrupted because the sandbox refused the synthetic loopback server; the
complete run in the permitted test environment passed.

Browser plugin was not available; repository Playwright used existing local Chrome.
Rendered evidence covers desktop and narrow layouts with synthetic HTTP, not Safari
or a paired real phone/iPad. No native popover behavior is claimed by these tests.
