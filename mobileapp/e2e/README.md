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
- Node ≥ 20. The toolchain is already in devDependencies (`appium`,
  `appium-xcuitest-driver`, `webdriverio`, `ts-jest`). One-time, idempotent:
  `npx appium driver install xcuitest`.

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

## What's covered

`smoke.test.ts`:

- **Always** — launch → the connection page's "Add a server" sheet → type an
  unreachable host → tap Connect → assert the "can't reach" error. Proves the
  whole toolchain plus a real type/tap/assert round-trip.
- **Gated on env** — pair against a live `gtmux serve` and assert the radar
  renders. Run it locally with a real token (kept out of the committed test):

  ```sh
  GTMUX_E2E_URL=http://127.0.0.1:8765 \
  GTMUX_E2E_TOKEN="$(cat ~/.config/gtmux/serve-token)" \
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
