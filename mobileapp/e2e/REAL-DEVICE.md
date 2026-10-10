# Physical iPhone acceptance

[中文](REAL-DEVICE.zh.md) · [simulator harness](README.md)

Use this procedure before declaring a connected iPhone accepted. An installed
version, a successful launch and a developer-service check are three separate
facts; none proves layout, interaction or VoiceOver. Record the app version,
iPhone model, iOS/Xcode versions, each scenario, screenshots and remaining gaps.

## 1. Inspect once, then choose the supported path

Check free space (`df -h /private/tmp`) and list devices with
`xcrun devicectl list devices`. For the selected device, read
`xcrun devicectl device info details --device <device>` and, if needed,
`xcrun devicectl device info lockState --device <device>`.
Do not poll in the background waiting for unlock. Report the exact required
user action if a service says the device is locked, untrusted or Developer Mode
is disabled. `unlockedSinceBoot` alone does not prove it is unlocked now.

| Path | Observed constraint | Next step |
| --- | --- | --- |
| `idevicescreenshot` (libimobiledevice) | Calls `screenshotr`; on iOS 26.6.2 it returned `Invalid service` even with `ddiServicesAvailable:true` | Keep the error once; do not infer missing DDI or a locked screen from its generic advice. Use a supported path below. |
| Xcode Device Hub → View Screen | This Mac's Xcode 27 requires iOS 27+ for screen sharing | Read the displayed requirement; do not retry it on iOS 26. |
| Appium/XCUITest + WebDriverAgent (WDA) | Worked on the same iPhone/iOS 26.6.2 with Xcode 27 | Use for screenshots, controls and scoped navigation. WDA is a small signed test helper, not a gtmux rebuild. |

These are measured compatibility results from 2026-10-10, not permanent version
rules. Capture actual tool errors again when versions change.

## 2. Preserve the user's phone and other workers

Use separate Appium and WDA ports and an owned DerivedData/artifact directory.
Do not run the simulator global setup on a personal phone: it reclaims processes
and writes debug flags. Do not reset pairings, write `gtmux-debug-flags.json`,
auto-accept confirmations, reinstall gtmux or send to working panes as setup.
Use the existing paired app for read-only navigation. Use an owned fixture/Demo
for changes in authorization; do not grant real follow/notification/knowledge
permissions just to test a switch. Keep screenshots/XML local; they can contain
conversation text or unrelated notifications. Do not commit them unredacted.

Start the installed Appium server on an unused port:

```sh
appium --port 14723 --log /private/tmp/<owned-run>/appium.log --log-level info
```

Create a session via `POST http://127.0.0.1:14723/session` with the W3C envelope
`{"capabilities":{"alwaysMatch":<caps>}}`. The following capabilities were
verified; select the device and an existing authorized signing team explicitly:

```json
{
  "platformName": "iOS",
  "appium:automationName": "XCUITest",
  "appium:udid": "<physical-device-UDID>",
  "appium:bundleId": "com.gtmux.app",
  "appium:noReset": true,
  "appium:autoLaunch": false,
  "appium:newCommandTimeout": 300,
  "appium:xcodeOrgId": "<authorized-team-ID>",
  "appium:xcodeSigningId": "Apple Development",
  "appium:updatedWDABundleId": "<provisioned-helper-bundle-ID>",
  "appium:derivedDataPath": "/private/tmp/<owned-run>/wda-dd",
  "appium:wdaLocalPort": 18100,
  "appium:wdaStartupRetries": 0,
  "appium:wdaLaunchTimeout": 60000,
  "appium:wdaConnectionTimeout": 60000,
  "appium:skipLogCapture": true
}
```

Record the returned session ID and startup/signing failure if any. Allow bounded
initial helper startup; do not repeatedly rebuild on a missing receipt. See the
installed driver's capability documentation before adding provisioning options.
The webdriverio harness needs Node 22 (see README); a direct HTTP client avoids
that client dependency for an ad-hoc run.

Set `POST /session/<id>/appium/settings` to
`{"settings":{"defaultActiveApplication":"com.gtmux.app"}}`.
An unrelated notification can make WDA element searches target SpringBoard even
while an app-specific source shows gtmux. Pin the target and re-read the source
instead of treating a failed selector as a product defect or blindly tapping.

Pinning the query does not prevent system notifications from intercepting touches.
Before a gesture, confirm the foreground app with `mobile: activeAppInfo` and
inspect the current screen. Stop device actions when the user switches apps.

Use `GET /session/<id>/screenshot` (base64 PNG) and `GET .../source` (XML).
Pair a screenshot with its control tree after the interaction settles. Inspect
the recorded screen before acting; prefer accessibility IDs from `testIds.ts`.
A terminal with long scrollback can make XML collection slow; retain bounded
HTTP timeouts and report an incomplete read. Missing attributes or a truncated
snapshot do not prove a control is absent.

## 3. Acceptance and cleanup

- Top preview/full reader: opens without growing terminal chrome; full text
  scrolls independently; close returns to the conversation.
- Radar: check the bottom and all-folded state; compare section counts and the
  footer's hidden-session count. Restore folds changed during the test.
- Follow settings: distinguish on/off/disabled; rows stay in place. Verify changes
  against an owned fixture, not by silently changing a user's authorization.
- HQ disc: long press, then **both** Usage and Knowledge; open, content, close and
  return. Inspect readable labels and button roles separately from screenshots.
- VoiceOver: actual focus, utterances and custom actions are a separate check.
  This installed driver's VoiceOver speech/control commands require iOS 27+;
  on iOS 26 ask for manual Settings → Accessibility → VoiceOver verification.
  An AX tree is evidence of labels/roles, not proof of spoken output or focus order.
- iPad, landscape, larger text and another language each need their own result;
  do not extrapolate from one portrait iPhone. A source fix not installed on the
  device must be reported as pending device acceptance.

Delete the test session (`DELETE /session/<id>`), stop only this run's Appium
process and verify its WDA process has stopped. Remove only this run's disposable
helper build directory after termination. Keep the evidence/receipt until reviewed
and report actual remaining space. Never reclaim another worker's ports or builds.
