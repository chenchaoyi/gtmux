# Manual captures and diagnostics

[中文](MANUAL-TOOLS.zh.md)

`npm run test:e2e` collects only `e2e/__tests__`. RN Jest excludes those device
suites and also excludes `capture/` and `diagnostics/`. Tools in the latter two
directories are preserved, but their screenshots/logs are not regression passes.

Use the existing owned simulator setup and explicitly select a tool category:

```sh
GTMUX_E2E_MANUAL=1 GTMUX_E2E_TOOL=capture npm run e2e:manual -- --testPathPatterns=hq-header-shot
```

Existing URL/token/UDID requirements still apply. Capture can contain private
screen content: keep evidence private. Knowledge capture never applies promotions.

The send reproduction requires `GTMUX_E2E_TOOL=diagnostics`,
`GTMUX_E2E_OWNED_FIXTURE=1`, the **exact** `GTMUX_E2E_PANE`, the fixture URL/token and
an owned simulator. The endpoint must be an HTTP loopback origin; its health must
identify `gtmux-e2e-fake-serve`, and its authenticated agent list must contain that
pane exactly once. Validation happens before device access. The fake server keeps
sends in memory; it does not input into real tmux panes. Pointing this diagnostic at
a real service is deliberately unsupported. It never selects the first radar row.

The keyboard probe diagnoses driver input and logs observations; it is not a
keyboard/shortcut acceptance assertion. An operator can inspect those observations
and record manual results, with source/build/device and limitations.
