# End-to-end UI tests (Appium / XCUITest)

For an already installed personal iPhone, follow [physical-device acceptance](REAL-DEVICE.md) ([中文](REAL-DEVICE.zh.md)) instead of running the simulator global setup.

These drive the gtmux iOS app the way a person does — launch it, type, tap,
assert what's on screen. They catch what the Jest unit tests can't: wrong text
in a list, a tap that doesn't fire, navigation landing on the wrong screen.

XCUITest injects touches through iOS's own automation framework (via
WebDriverAgent), so it works where raw synthetic clicks don't.

## Prerequisites

- A booted iOS simulator. Default target: **iPhone 17 Pro / iOS 26.5**
  (`e2e/setup/capabilities.ts`; override with `GTMUX_E2E_DEVICE` / `GTMUX_E2E_OS`
  / `GTMUX_E2E_UDID`).
- **Node 22.11+ within the 22.x line.** `package.json` requires at least 22.11;
  Node 20 does not satisfy it. The harness has a recorded Node 26 failure: webdriverio's request
  fails inside undici before it leaves the client (`UND_ERR_INVALID_ARG` on
  `POST /session`, with nothing in the Appium log but the `/status` probe). Use
  `nvm use 22` (or prefix `PATH`) for `test:e2e`. The toolchain is in devDependencies (`appium`,
  `appium-xcuitest-driver`, `webdriverio`, `ts-jest`). One-time, idempotent:
  `npx appium driver install xcuitest`.
- **Software keyboard for typing tests.** Set `GTMUX_E2E_SOFT_KEYBOARD=1` for the
  session; the harness sets `connectHardwareKeyboard:false` and
  `forceTurnOnSoftwareKeyboardSimulator:true`. Verify text entry after focusing
  the field: a successful `setValue` call alone does not prove it received text.

## Run

```sh
cd mobileapp          # from the repository root
npm run e2e:build      # build Release and reinstall on the selected test simulator
GTMUX_E2E_SOFT_KEYBOARD=1 npm run test:e2e
```

`npm run e2e:build` re-installs the app; it is not a guarantee that Keychain
pairings are empty. `smoke.test.ts` explicitly uses `GTMUX_DEBUG_RESET_SERVERS=1`
to clear saved servers in its test app. Fake-backed suites can auto-pair to their
fixture. Use a dedicated test simulator. Rebuild after an app source change —
the e2e session does **not** rebuild the app (`noReset:true`); WebDriverAgent may
still need its own build.

`npm run test:e2e` does it all in one shot: spawns an Appium server (log →
`.e2e-artifacts/<run>/appium.log`), waits for `/status`, opens a webdriverio
session on the selected sim, discovers `e2e/__tests__/**/*.test.ts`, then closes
the session and reclaims the Appium/WDA processes. Individual suites may skip
without their required flags; the coverage reporter lists those that ran and
those that did not. A successful exit is not full-suite or real-device acceptance.

To drive Appium ad-hoc, run the server alone: `npm run e2e:appium`, then connect
your tool to `http://127.0.0.1:4723`.

## Debug flags

The native `DebugSettings` module reads `Documents/gtmux-debug-flags.json` and
then overlays any `GTMUX_DEBUG_*` launch environment values. The harness's
`launchWithFlags` writes that file before terminating and reactivating the app;
it does not rely on `mobile: launchApp` environment updates. The file persists
across a plain relaunch. For an unflagged launch, write an empty flags object
and relaunch without debug environment values. Use synthetic credentials in
this test-only data container.

| env var | effect |
| --- | --- |
| `GTMUX_DEBUG_PAIR_URL` + `GTMUX_DEBUG_PAIR_TOKEN` | auto-pair on launch (in-memory; skip the manual pairing screen) |
| `GTMUX_DEBUG_NO_PUSH=1` | skip the push-permission prompt (it otherwise blocks UI tests) |
| `GTMUX_DEBUG_RESET_SERVERS=1` | clear this test app's saved servers on launch |
| `GTMUX_DEBUG_RESET_UI_STATE=<token>` | start from the fixture view state (the keys in `src/state/uiState.ts` `E2E_FIXTURE_KEYS`, today the radar's folds), once per token. `writeDebugFlags` sends one token per test FILE, so every file starts unfolded and a relaunch inside a file keeps what it set; pass `''` to inherit. Added after `radar-refresh-collapsed` left every section folded and `edge-states` could not find its rows (F18, 2026-10-06) |
| `GTMUX_DEBUG_LOG_NET=1` | record requests through the API client's fetch wrapper (`method · path · status/error · ms`; no token/body fields) to `Documents/gtmux-debug.jsonl` |

`readDebugLog()` (`e2e/setup/app.ts`) reads that JSONL back via
`xcrun simctl get_app_container`, so a test can assert on the **network layer**
the UI drove, not just the pixels. This is not a complete trace of every network
path: enrollment and the SSE subscription have separate implementations.

## Suite examples and coverage limits

- `smoke.test.ts` — launch → the connection page's "Add a server" sheet → type an
  unreachable host → tap Connect → assert the "can't reach" error. Proves the
  toolchain plus a type/tap/assert round-trip. (A second test pairs against a
  live serve if `GTMUX_E2E_URL`/`TOKEN` are set.)
- `radar.test.ts` — launches against the fake serve with the debug layer (auto-pair +
  no-push + net-log), drives **radar → open a pane → Detail → back**, then
  asserts the recorded log shows `/api/agents` + `/api/pane` succeeded. Other
  4xx/5xx fail except the explicitly tolerated `/api/awake` 404. This exercises
  the app against the fake server, not real tmux or APNs.

- `edge-stability.test.ts` (gated on env) — drags into terminal scrollback and back to
  the live tail, then **stands still** and reads the terminal's own per-frame probe. The
  top chrome folding changes the viewport, and the viewport is what "am I at the tail" is
  measured against, so an unstable header shows up as the viewport sawtoothing while
  nothing is moving. It asserts that once the reveal starts, the viewport only shrinks.
  Measured before the 2026-09-05 fix: `692.7 → 619.7 → 628.7 → 576`. After: monotone.
  The rule it guards is `src/ui/liveEdge.ts`.

### The fake serve and its seeds

`fake-serve/` is an in-process stand-in for `gtmux serve` (`startFake()`), and it is what
a suite should run against rather than a real machine's serve and its panes. Its stock world
is small on purpose. A suite that needs more seeds it, opt-in, in its own `beforeAll`:

| seed | what it adds | used by |
| --- | --- | --- |
| `seedLongHistory(id, lines)` | a terminal with hundreds of lines of history | `terminal-scroll-collapse` |
| `seedBusyPane(id, lines)` | a long terminal that prints a line on every capture | `jumpbottom` |
| `seedLongChat(id, turns)` | a conversation of multi-paragraph replies | `chat-fullscreen-collapse`, `hq-chrome-stability` (HQ, `%6`) |
| `seedCalls(n)` | `n` sessions waiting on you, so "Your call" scrolls | `hq-chrome-stability`, `hq-header-collapse` |
| `seedShell(session)` | a plain shell the pane browser lists, whose input line takes keys | `composer-keyrow` |
| `seedUsage()` | the full `/api/usage`, windows grouped per agent | `usage-sheet` |
| `seedCursor(id, cursor?)` | a `cursor` on `/api/pane` | `cursor` (manual, `GTMUX_CURSOR=1`) |
| `seedDemo(lang)` | the in-app Demo's whole world (rows, panes, screens, chats, digest, board, ledger, knowledge, usage, theme), read from the Demo's own client | `site-shots` |
| `seedIcons()` | an `icon` hint on every agent row whose mark `/api/icon` can serve | `site-shots` |

`/api/icon` answers `<key>.png` from `GTMUX_FAKE_ICON_DIR` when it is set, else from the
repo's `assets/agent-icons`, by label or by key as the real serve does, and 404s otherwise.
The repo has no `claude.png`; the website captures (`site-shots`, see
`docs/appstore-shots.md` §5) point the variable at a local directory that has one.

`fake-serve/seeds.test.ts` (part of `npm run check`) checks each seed through the app's
own client and screen models; `fake-serve/demo.test.ts` holds `seedDemo` against the Demo
itself, and `fake-serve/icons.test.ts` covers `/api/icon` and `seedIcons`. `fake-serve/contract.test.ts` compares the fake's response
shapes, seeded ones included, with a real serve's; it reads only, and skips unless
`GTMUX_E2E_URL`/`GTMUX_E2E_TOKEN` are set.

Suites still gated on a real serve take its address and token from the environment.
Use an isolated test HOME and tmux socket with disposable panes; inspect the selected
suite's actions first. Do not point a write test at a working session or copy the
normal user's serve token. With an owned fixture already running, set its values:

```sh
GTMUX_E2E_URL="${AUDIT_SERVE_URL:?set the isolated fixture URL}" \
GTMUX_E2E_TOKEN="${AUDIT_SERVE_TOKEN:?set the synthetic fixture token}" \
GTMUX_E2E_UDID="${AUDIT_SIM_UDID:?set the owned simulator UDID}" \
npm run test:e2e -- smoke
```

## Conventions

- **Accessibility-id targeting.** Selectors use `~<id>` where the id is a RN
  `testID` (→ iOS `accessibilityIdentifier`), sourced from
  `src/constants/testIds.ts` so a rename refactors both sides. Prefer this over
  visible text — the UI is bilingual (en/zh), text isn't stable.
- **Artifacts** land in `.e2e-artifacts/<run>/` (gitignored), with
  `.e2e-artifacts/latest` symlinked to the most recent. On failure a test writes
  `fail-<label>.png` + `fail-<label>.xml` (the accessibility tree at that
  moment — invaluable for figuring out why a selector missed).
- **Alerts are the run's explicit choice** (`GTMUX_E2E_ALERTS`, `e2e/setup/capabilities.ts`).
  `dismiss` (the default) cancels any alert nobody asked about, so a stray system prompt
  cannot wedge the run; `accept` accepts instead, for a run that needs the push permission
  (`GTMUX_E2E_ACCEPT_ALERTS=1` still means this); `manual` touches nothing. Both automatic
  modes also catch the APP's own `Alert.alert` confirmations, about a second after they
  appear, so a test that taps Revoke and asserts what happens must run in `manual` and
  answer the alert itself with `answerAlert(button)`, which logs every answer. The session
  line in the run's output names the mode in force.

## Layout

```
e2e/
├── README.md
├── tsconfig.json              # independent Node/CommonJS test configuration
├── jest.config.e2e.js         # ts-jest, node env, global setup/teardown
├── setup/
│   ├── capabilities.ts        # sim caps + Appium URL (env-overridable)
│   ├── driver.ts              # globalThis singleton accessor
│   ├── global-setup.ts        # spawn Appium, open the session
│   ├── global-teardown.ts     # close session, group-kill server
│   └── screenshot.ts          # screenshot + on-failure page-source dump
└── __tests__/
    ├── smoke.test.ts
    ├── radar.test.ts
    └── …                     # see the run's coverage report for executed suites
```

## iPad

The same harness, a different simulator: boot an iPad (the store slot is `iPad Pro 13-inch (M5)`)
and pass it by UDID and name — the device name is what gates the iPad suites.

```sh
GTMUX_E2E_UDID="${AUDIT_IPAD_UDID:?set the owned iPad simulator UDID}" npm run e2e:build
GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' \
  GTMUX_E2E_URL="${AUDIT_SERVE_URL:?set the isolated fixture URL}" \
  GTMUX_E2E_TOKEN="${AUDIT_SERVE_TOKEN:?set the synthetic fixture token}" \
  npm run test:e2e -- split-shell
GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- ipad-demo
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" \
  GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad
```

`GTMUX_DEBUG_LANG=en|zh` (a launch flag) forces the app's language for a capture, so the
two locales' screenshots come from one simulator without changing its locale.

Hardware keyboard: the 2026-09-12 investigation reached command registration but
XCTest's simulator injection did not dispatch the commands. Keep that result
separate from a fresh run: `ipad-keys` currently asserts navigation after injected
keys, and a failure still needs diagnosis. Physical-keyboard dispatch requires
separate device acceptance; registration logs alone do not verify it.


### Test-only TypeScript configuration

The e2e harness is Node/CommonJS and has its own `tsconfig.json`; it does not inherit React Native's bundler resolution or custom conditions. Check it with `npx tsc --project e2e/tsconfig.json --noEmit`. WebDriver's chainable arrays have asynchronous metadata: use `.getElements()` before synchronous length/index access, and await element IDs passed to lower-level commands.

`new-session.test.ts` uses the fake Mac to check a keyboard-up single tap, duplicate-name recovery, one actual creation and navigation to the returned terminal. Do not hide the keyboard or retry the first tap to make this regression pass.
