# Test system

[中文](TEST-PLAN.zh.md) · [Case catalogue](CASES.md) · [Design workflow](DESIGN-WORKFLOW.md) · [Design traceability](DESIGN-TRACEABILITY.md)

Test **layers describe the boundary being exercised**. Go, React Native (RN), Swift
and Cloudflare Workers describe implementations and runtimes; they are not parallel
test layers. One technology can appear in several layers. Build success, static
checks, screenshots and device interaction each prove different things.

## Layers

| Layer | Question and observable result | Examples in gtmux | Environment / limitation |
| --- | --- | --- | --- |
| L1 — unit / pure logic | Does an isolated parser, model or policy return the right result for normal, boundary and invalid input? | Go classification/history/restore policy; RN Trace models and formatters; Swift HTML/Markdown preprocessing; Android document policy | Synthetic data, clock/network injected as needed; does not prove a screen or native gesture |
| L2 — component / contract | Does a component or boundary consume the right data and expose the right behavior? | RN rendered components with press callbacks; Go `httptest` route contracts; Worker request/response and APNs/FCM payloads; Swift stores with fake clients | RN renderer is not an installed app; Go route tests do not imply the whole daemon is running |
| L3 — runtime integration | Do multiple pieces cooperate in an actual runtime? | Chromium loads real embedded HTML/JS/CSS against a synthetic API; Cloudflare runtime tests; native bridge integration when explicitly run | Browser fixtures are not real Go-server contracts. Foundation command-line oracles remain L1, not WKWebView interaction |
| L4 — application journeys | Can a user complete an application flow through real compiled UI and a device driver? | Android phone/tablet emulator journey; explicitly enabled iOS Appium regression suites | Fixture-backed UI exercises the app, not production APNs, real agent input or all OS permissions |
| M — manual / device acceptance | Is the actual layout, touch, focus, accessibility speech and OS integration usable? | Real iPhone/iPad, Safari, VoiceOver/TalkBack, hardware keyboard, rotation/split view, Ghostty focus | Record device, source/build, steps, result and limitations. Screenshot or AX tree alone is not interaction/speech proof |
| G — qualification gates (orthogonal) | Is source/artifact policy satisfied? | Typecheck, lint, race/vulnerability checks, strict specs, bilingual/generated docs, source seals, architecture, signing/notarization/package validation | A gate may protect any layer. Compilation, regex or signature checks are not behavioral UI tests |

Choose the lowest layer that detects the fault, then add a higher-layer regression
when integration or presentation caused the real failure. Keep meaningful legacy
provider compatibility and permission/ownership regressions. No fixed pyramid
percentage or test-count target substitutes for an effective assertion.

## Where each product and technology fits

| Implementation / surface | Typical layers | Actual runners / sources |
| --- | --- | --- |
| Go core, CLI, HTTP server, HQ/restore | L1 + L2; isolated subprocess/service integration L3; G race/build/policy | `go test -race ./...`, `internal/**`, `cmd/**`, `relay/**`; Linux user-service acceptance owns a temporary account |
| RN shared app (iPhone, iPad, Android phone/tablet) | Pure TS L1; React renderer L2; bridge integration L3; installed UI L4 | `mobileapp/src`, Jest; Android journey and opt-in Appium. RN is the shared framework, not an additional layer between UI and Go |
| iOS Swift / Android Kotlin or C++ | Logic L1, platform/bridge L3 when run, app L4; build/package G | Foundation document oracles, Widget target checks, Android JUnit/lint/QA package, unsigned ARM64 app/extensions build |
| macOS menu bar / Widgets | Models L1, stores/components L2, actual window/system operation L3/M; universal build G | `macapp/Tests`, `shared/UsageWidgets`; actual focus and UI acceptance separately recorded |
| Browser | Models L1; DOM/browser runtime L3; real-host acceptance L4/M when explicitly run | Node tests in `internal/server/web`, Chromium in `tests/web` |
| Workers (push relay, tunnel, download) | Handler/contract L2, Worker runtime L3, deployed smoke when separately authorized; type/policy G | `relay-worker`, `tunnel-worker`, `download-worker`, `shared/worker-json.test.mts`. A Worker is an edge service runtime, not a test level |
| CI, delivery, docs and test infrastructure | L1/L2 tests of tooling; G for the policies those tools enforce | `scripts/*_test.py`, `mobileapp/scripts`, Ruby/Shell/native harnesses and `internal/docs` |

The browser and RN use the same semantic behavior fixture
[`tests/contracts/client-behavior.json`](../../tests/contracts/client-behavior.json).
Assertions execute each real renderer independently: five providers, original turn
151, counts, initially folded turns, individual calls, HQ sender identity, Markdown
and scoped file actions. Shared helpers alone do not prove final UI parity. These
fixtures do not invent a browser HQ-workspace screen or native WK/gesture coverage.

## CI stages

The authoritative entry points are [CI](../../.github/workflows/ci.yml),
[scope selection](../../scripts/ci.py) and the six workflows listed below. The
matrix describes what runs, not a claim that a particular revision passed.

| Stage | Tests / checks executed | What does not follow from success |
| --- | --- | --- |
| PR to main | Always scope + quality + `ci-required`; affected platform jobs below. Test-design history checks require the sealed review before the first test edit | No signing, deployment, installation or implicit human approval |
| Actual main push | Same conservative selector and selected jobs; validate catalogue/design snapshot after squash; separate run bound to actual merge SHA | A PR pass is not this run; a squash cannot preserve pre-implementation commit order in its single resulting commit |
| Manual `ci` dispatch | All platform suites plus quality and aggregate | Does not install or publish |
| Tag / release (`release.yml`) | Resolve immutable source/definition; Go security and complete `make check`; cross-build CLI; universal signed Mac build, notarization/staple verification; publish only after required producers; signed feed/mirror checks | CLI/Mac publication does not mean an iOS/Android upgrade or actual installation |
| iOS test delivery (`ios-test-delivery.yml`) | Exact reviewed main/tag and successful matching main CI; scope comparison can skip unchanged iOS inputs; protected signing/publisher preflight, build/archive/export and artifact checks; preview and release-test modes differ | No Store Submit; uploaded IPA or Safari link is not proof of installation/UI acceptance |
| Android test delivery (`android-test-delivery.yml`) | Exact source on main, successful native **and both emulator** checks; build ordinal/input checks; unsigned production build and encrypted artifact; operator subsequently verifies/signs/publishes under its delivery procedure | QA APK is not a signed release; uploaded APK is not a physical-device pass or Play listing |
| Store assets (`ios-store-assets.yml`) | Validate source; build/reuse validated simulator host; capture missing locale/device groups, checkpoint, render/verify assets | Capture is material production, not automatic functional/visual approval or Store submission |
| Mirror recovery (`release-mirror.yml`) | Validate published version and frozen installer/source, signed feed/trust, sync existing immutable assets | No new product build; cannot substitute for original release or install evidence |

There is no scheduled blanket UI/nightly workflow in this checkout. Longer exploratory
and real-device runs require explicit recording; do not imply they run on every PR.

### Selected CI jobs and commands

| Job | Checks and test layer | Selection / environment |
| --- | --- | --- |
| `quality` | `make ci-test publisher-test`; actionlint; `check-design.sh --source-only` including test design/catalogue and bilingual pairs; strict OpenSpec; generated assets/docs; download-worker test/build; `go test ./internal/docs` | Every PR/main/manual run; private exact-term policy required for owned runs, explicitly unavailable to forks |
| `test` | gofmt/vet/staticcheck, isolated tmux prerequisite; Go `-race -coverprofile`; Linux user-service acceptance; vulnerability protocol and design checks (L1/L2/L3 + G) | Go/core or affected Web/relay consumers; unknown/shared impact selects all |
| `cross-build` | CGO-free CLI cross-compilation | Selected core/shared impact; build qualification, not runtime tests |
| `mobile` | `npm ci`, tsc, eslint, complete Jest, delivery-helper tests, creative-layout checks (L1/L2 + G) | RN or downstream consumers; synthetic bridge mocks do not prove native UX |
| `web` | Complete `internal/server/web/*.test.cjs` Node tests, Chromium rendered tests (`tests/web`) (L1/L3) | Embedded Web/browser/shared mobile presentation changes |
| `ios-native` | Production Swift HTML/Markdown oracles via `check-native-documents.py` (L1); Ruby Widget boundary; unsigned Release ARM64 app + two embedded extensions compile/architecture (G) | iOS/RN/shared changes; no device boot, UI automation, signing or upload |
| `android-native` | `assembleQa testQaUnitTest lintQa`; source seal, package/ABI/16KiB checks (L1 + G) | Android/RN/shared input changes |
| `android-emulator` | Phone and tablet matrix consuming the exact source-sealed QA APK; actual UI journey (L4) | Always selected with Android build; no shared local emulator/ADB |
| `menubar` | Swift universal build, UsageWidgets and menu-bar Swift tests, Widget build/design (L1/L2 + G) | Menu-bar or shared/uncertain changes; native visual/focus acceptance separate |
| `relay-worker` | Typecheck, shared JSON/APNs/FCM payload and handler tests (L2 + G) | Push source/tests and downstream clients |
| `tunnel-worker` | Typecheck, Node contract + Worker runtime Vitest tests (L2/L3 + G) | Tunnel/shared/uncertain changes |
| `ci-required` | Fail-closed aggregation, including both Android matrix results | Every run; required skip/missing/failure/cancellation fails; intentional unselected skip is not a pass |

Mixed changes take the union. Unknown paths, dependencies, workflow/build changes,
invalid/unavailable diffs and symlink changes broaden to all. Renames include both
paths. See [CI rules](../ci.md); do not manually narrow scope to obtain green checks.

## Case lists and design-to-implementation rules

[`CASES.md`](CASES.md) explains the layer indexes; [`CASES.csv`](CASES.csv) lists
every inventoried declaration/subtest/template with stable ID, layer, technology,
source/line, description and review origin. [`catalogue.json`](catalogue.json)
pins source hashes. Counts are **static declaration positions**, not expanded
parameter instances, executed tests or passes. Legacy structural classification
needs semantic review when touched; it is not retroactive design approval.

For any added, changed, moved or removed test: follow [the design workflow](DESIGN-WORKFLOW.md).
Record intended observable behavior **first**, review and commit the sealed design,
then implement and map concrete selectors. `make test-design` verifies the resulting
snapshot; PR CI additionally verifies ordering. Keep design changes and implementation
changes in separate commits until CI has evaluated them; do not squash locally first.
The normal final GitHub squash is supported: main checks the final snapshot.

A review seal protects against accidental drift. It is not a cryptographic identity
or proof that an assertion is useful. Reviewers still check requirements, failure
oracles, mocking boundaries and scope, and rerun a deliberate negative example.

## Manual acceptance and reporting

Retain the [surface design matrix](DESIGN-TRACEABILITY.md). For Mac check state glyphs,
light/dark/tinted menu bars, search, preferences, keyboard and exact pane focus. For
phone/tablet check pairing, HQ/knowledge, input, native document previews, long Trace,
rotation/split view and accessibility. For Web check narrow/wide, enrollment, revocation,
failed-send draft retention, live refresh, history anchors and external sessions.

Capture/probe tools live in `mobileapp/e2e/capture` and `diagnostics`; ordinary RN and
Appium regression collection excludes them. Explicit `e2e:manual` requires the named
tool opt-in. Send repro additionally verifies the exact pane and loopback fake-server
identity before accessing a device; capture does not apply knowledge promotions.
See [manual tools](../../mobileapp/e2e/MANUAL-TOOLS.md).

Reports separate design reviewed, code implemented, tests run, deliberate skips,
compile-only, actual simulator actions, real device/accessibility and installation.
Do not inherit a prior report's passes or turn a screenshot into touch/VoiceOver proof.
