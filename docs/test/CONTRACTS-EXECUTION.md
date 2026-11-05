# Client contracts and isolated execution

This extends [the layered test system](TEST-PLAN.md). Reviewed cases and source
selectors remain in the [case index](CASES.md); counts are not runtime passes.

## Hermetic HTTP contracts (L2)

`tests/contracts/http-contract.json` contains synthetic store inputs, HTTP requests
and the observed real Go handler responses. `TestClientHTTPContracts` executes the
production authenticated routes with injected stores and a temporary HOME. Ordinary
Go CI compares the committed responses; deliberate changes require review and the
explicit `GTMUX_UPDATE_HTTP_CONTRACT=1` regeneration flag. Never regenerate merely
to silence a failure. HQ reads execute the actual application projection on an isolated store; only its
two volatile overview clocks are fixed. This does not start a live tmux daemon.

RN `e2e/fake-serve/backend-contract.test.ts` and Web `backend-contract.test.cjs`
independently serve the same domain inputs and compare actual HTTP responses.
Neither fake reads the expected response. The shared helper only makes requests
and asserts results. Covered: original Trace ordinals, history and stale cursors,
context opt-in/guest redaction, conditional reads, empty lists, HQ overview/detail
and owner scope, document metadata/omitted optional home, download bytes and security
headers, unknown request fields and unauthenticated access. A negative check rejects
mutated types, missing fields/headers and permission/count drift.

The optional existing live-server shape suite is separate. It still skips without
explicit environment configuration; these new hermetic checks do not need a user
server, token, real pane or native build.

## Mounted interactions (L2)

The new screen suites mount production Detail/Composer, HQ overview/matter/console/
Composer, and AgentsProvider/ManageMacScreen. Synthetic clients and deferred promises
exercise server/session draft isolation, delayed old responses, actual subscription
notifications, question-to-original-matter navigation and prepared reply submission.
An authoritative saved answer withdraws the human decision card into HQ follow-up;
it does not mark the ongoing matter completed. Historical question text may remain
in follow-up. The new regression also found and fixed a prepared HQ reply crossing
server/session switches: navigation and prefill now use the same scoped identity as
the composer, without carrying the old question into a new console. No send occurs merely from selecting a choice.

## Android capability blocks (L4, scheduler policy at L1)

The default `npm run test:e2e:android` keeps all 31 existing UI checks on each owned
phone/tablet emulator. `GTMUX_ANDROID_E2E_BLOCKS=trace,hq,documents,permissions`
selects those capabilities; available names are declared in `e2e/android/journey.cjs`.
`GTMUX_ANDROID_E2E_ONLY=08` is a developer subset and automatically includes required
06/07 prerequisites. Empty/unknown selectors fail. CI continues to run the full journey.

Each group either starts with its existing QA-app launch or has an explicit launch/
entry setup. Independent groups do not depend on the previous group's final page.
Dependencies such as folded-history and HTML-to-Markdown return are explicit.
Failed setup/required checks produce `blocked` with the first cause. A disconnected
driver records one root failure and marks later checks `not-run`; neither status is a
pass. An independent group remains eligible after an ordinary assertion failure.
No automatic retry hides a failure. Only the named owned emulator and QA app are reset.

`RESULTS.json` schema 2 retains `results` and adds selection, capability/group,
`blocked_by`, action/evidence durations and stage/operation metrics: APK install,
driver session, app launch, scrolling, page-source reads, evidence and tap settling.
Metrics overlap (e.g. page-source within evidence): do not add them into a wall-clock
sum. Assertions, all passing screenshots/source captures and independent failure
capture remain enabled. This change does not claim Android is faster.

## Browser process reuse and CI scope (L3 + G)

One Chromium process is reused per Node test worker. Every case still gets a new
BrowserContext, page, tracing session and synthetic HTTP server, closed on success
or failure. Crashed processes are replaced for subsequent cases; the current failed
case is never retried. Cookie/storage and fixture-state isolation have rendered
regressions. `GTMUX_WEB_FRESH_BROWSER=1` retains a fresh-process comparison mode;
`GTMUX_WEB_TIMING=/absolute/output` writes per-worker timing JSONL. Failure screenshot,
trace and console/title checks remain unchanged.

Only `mobileapp/src/**/*.test.ts(x)` may omit native gates, and only when `ci.py`
verifies the full immutable Metro configuration hash that blocks test-module imports.
Those changes run complete RN and Web suites plus the always-on policy gate. Missing
proof retains native checks. E2E/fake server changes, contract data, build/config,
dependencies, mixed product edits and unknown paths keep their existing broader scopes.
This prevents test-only naming from becoming an unchecked product-build exemption.

No simulator/component result proves physical-device touch, VoiceOver/TalkBack,
hardware keyboard, native document rendering or Safari acceptance.
