# End-to-end UI tests (Appium / XCUITest)

These drive the gtmux iOS app the way a person does — launch it, type, tap,
assert what's on screen. They catch what the Jest unit tests can't: wrong text
in a list, a tap that doesn't fire, navigation landing on the wrong screen.

XCUITest injects touches through iOS's own automation framework (via
WebDriverAgent), so it works where raw synthetic clicks don't.

## Prerequisites

- A booted iOS simulator. Default target: **iPhone 17 Pro / iOS 26.5**
  (`e2e/setup/capabilities.ts`; override with `GTMUX_E2E_DEVICE` / `GTMUX_E2E_OS`
  / `GTMUX_E2E_UDID`).
- **Node 20–22.** Not newer: on Node 26 the session never opens — webdriverio's request
  fails inside undici before it leaves the client (`UND_ERR_INVALID_ARG` on
  `POST /session`, with nothing in the Appium log but the `/status` probe). Use
  `nvm use 22` (or prefix `PATH`) for `test:e2e`. The toolchain is in devDependencies (`appium`,
  `appium-xcuitest-driver`, `webdriverio`, `ts-jest`). One-time, idempotent:
  `npx appium driver install xcuitest`.
- **Software keyboard on.** If the sim's hardware keyboard is connected, the soft
  keyboard never appears and `setValue` silently types nothing (the #1 gotcha).
  Disable it once, then reboot the sim:
  `defaults write com.apple.iphonesimulator ConnectHardwareKeyboard -bool false`
  (or Simulator → I/O → Keyboard → uncheck "Connect Hardware Keyboard").

## Run

```sh
npm run e2e:build      # build Release for the sim + install FRESH (clean Keychain)
npm run test:e2e       # spawn Appium, open a session, run e2e/__tests__/**
```

`npm run e2e:build` re-installs the app, so the suite starts on the connection
page. Re-run it after any source change — the e2e session does **not** rebuild
(`noReset:true`).

`npm run test:e2e` does it all in one shot: spawns an Appium server (log →
`.e2e-artifacts/<run>/appium.log`), waits for `/status`, opens a webdriverio
session on the booted sim, runs every `e2e/__tests__/**/*.test.ts`, then closes
the session and group-kills the server (including the WebDriverAgent xcodebuild
grandchild).

To drive Appium ad-hoc, run the server alone: `npm run e2e:appium`, then connect
your tool to `http://127.0.0.1:4723`.

## Debug launch-arg layer

For deeper, more convenient tests the app exposes a debug channel gated entirely
by `GTMUX_DEBUG_*` **launch environment** (native `DebugSettings` module →
`src/debug`). A normal launch sets none of these, so production is unchanged.
Appium passes them via `mobile: launchApp` (see `e2e/setup/app.ts`).

| env var | effect |
| --- | --- |
| `GTMUX_DEBUG_PAIR_URL` + `GTMUX_DEBUG_PAIR_TOKEN` | auto-pair on launch (in-memory; skip the manual pairing screen) |
| `GTMUX_DEBUG_NO_PUSH=1` | skip the push-permission prompt (it otherwise blocks UI tests) |
| `GTMUX_DEBUG_LOG_NET=1` | record every API call (`method · path · status · ms`; token/body never logged) to `Documents/gtmux-debug.jsonl` |

`readDebugLog()` (`e2e/setup/app.ts`) reads that JSONL back via
`xcrun simctl get_app_container`, so a test can assert on the **network layer**
the UI drove, not just the pixels.

## What's covered

- `smoke.test.ts` — launch → the connection page's "Add a server" sheet → type an
  unreachable host → tap Connect → assert the "can't reach" error. Proves the
  toolchain plus a type/tap/assert round-trip. (A second test pairs against a
  live serve if `GTMUX_E2E_URL`/`TOKEN` are set.)
- `radar.test.ts` — launches against the fake serve with the debug layer (auto-pair +
  no-push + net-log), drives **radar → open a pane → Detail → back**, then
  asserts the recorded log shows `/api/agents` + `/api/pane` succeeded with no
  4xx/5xx. A real user scenario exercised end-to-end, UI and network together.

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

`fake-serve/seeds.test.ts` (part of `npm run check`) checks each seed through the app's
own client and screen models. `fake-serve/contract.test.ts` compares the fake's response
shapes, seeded ones included, with a real serve's; it reads only, and skips unless
`GTMUX_E2E_URL`/`GTMUX_E2E_TOKEN` are set.

Suites still gated on a live serve take its address and token from the environment (kept
out of the committed tests):

```sh
GTMUX_E2E_URL=http://127.0.0.1:8765 \
GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
GTMUX_E2E_UDID=<booted-udid> \
npm run test:e2e
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
├── tsconfig.json              # extends ../tsconfig with node types
├── jest.config.e2e.js         # ts-jest, node env, global setup/teardown
├── setup/
│   ├── capabilities.ts        # sim caps + Appium URL (env-overridable)
│   ├── driver.ts              # globalThis singleton accessor
│   ├── global-setup.ts        # spawn Appium, open the session
│   ├── global-teardown.ts     # close session, group-kill server
│   └── screenshot.ts          # screenshot + on-failure page-source dump
└── __tests__/
    └── smoke.test.ts
```

## iPad

The same harness, a different simulator: boot an iPad (the store slot is `iPad Pro 13-inch (M5)`)
and pass it by UDID and name — the device name is what gates the iPad suites.

```
GTMUX_E2E_UDID=<ipad udid> npm run e2e:build
GTMUX_E2E_UDID=<ipad udid> GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' \
  GTMUX_E2E_URL=http://127.0.0.1:8765 GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
  npm run test:e2e -- split-shell        # the regular shell over a live serve
GTMUX_E2E_UDID=<ipad udid> GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- ipad-demo
GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG=en GTMUX_E2E_UDID=<ipad udid> \
  GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad
```

`GTMUX_DEBUG_LANG=en|zh` (a launch flag) forces the app's language for a capture, so the
two locales' screenshots come from one simulator without changing its locale.

Hardware keyboard: `ipad-keys` reaches the app and reads back that the 22 commands were
registered and the main menu built, but XCTest's key injection on a simulator types text
and never dispatches a `UIKeyCommand` (with or without a first responder, hardware keyboard
connected or not — measured 2026-09-12). The dispatch is checked by hand with a keyboard on
a device; on a simulator that suite is expected red.


### Test-only TypeScript configuration

The e2e harness is Node/CommonJS and has its own `tsconfig.json`; it does not inherit React Native's bundler resolution or custom conditions. Check it with `npx tsc --project e2e/tsconfig.json --noEmit`. WebDriver's chainable arrays have asynchronous metadata: use `.getElements()` before synchronous length/index access, and await element IDs passed to lower-level commands.

`new-session.test.ts` uses the fake Mac to check a keyboard-up single tap, duplicate-name recovery, one actual creation and navigation to the returned terminal. Do not hide the keyboard or retry the first tap to make this regression pass.
