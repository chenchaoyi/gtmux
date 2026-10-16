# Capture App Store images in Actions

When local disk space is limited, run **App Store screenshots (maintainers)**
(`ios-store-assets.yml`) from `main`, with a published stable `release_tag`.
The workflow verifies the release and successful main CI, then builds that exact
source for Simulator. It needs no signing, publisher or App Store credentials.

It reuses the existing demo capture and framing scripts. A dedicated iPhone 17 Pro
Max and iPad Pro 13-inch simulator each capture English and Chinese. Only simulators
created by the run are removed. The artifact contains 22 framed screenshots, two
creative images, the raw demo captures and a manifest of source IDs and image hashes.
The lock-screen image remains a rendering of the widget, not a real push test.

```sh
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z
```

Download the resulting `appstore-assets-<tag>-<run>` artifact privately. Inspect every
image at its original size. Copy `screenshots/<locale>/` and `creative/<locale>/` to
the corresponding `mobileapp/fastlane/` folders; retain the manifest with release
evidence. Follow [the submission runbook](submit.md) to place and read back all four
screenshot groups and the four Header/Search Results placements. This workflow
does not upload to App Store Connect or submit a version for review. Its images do
not establish real-device, VoiceOver, APNs or Live Activity acceptance.

The runner registers the XCUITest driver from npm at the version in the release's
lockfile. Do not register its installed `node_modules` folder with `--source=local`:
Appium treats that as a source checkout and runs `prepare`, but the published
package does not include the TypeScript project needed for that rebuild (TS5083).

Binary preparation is separate. See [reusing a verified release IPA](reuse-ipa.md)
for the additional signing and validation required when local disk space is limited.

## Capture harness and diagnostics

The app is built from the published tag. The reviewed workflow supplies only the
two capture tests, their demo selector and simulator capabilities; it never
overlays production sources. The manifest records both commits and the harness
file hashes. This lets the harness fix capture-only failures without changing the
released app. Unit tests render the real pairing screen to check the demo selector.

The runner keeps its prebooted simulator headless. Without `isHeadless`, Appium
shuts down a simctl-booted device to restart it with a visible Simulator window,
which timed out on the hosted runner. Local e2e runs keep their GUI behavior unless
they explicitly opt in. The capture helper refuses to run on a shared local Mac.
Structured progress names each device/language phase. Failure artifacts explicitly
include the allowlisted hidden `.e2e-artifacts` Appium logs and demo PNGs.

The current visible demo entry is **No server handy? See a demo** in English and
**尚未连接？看看演示** in Chinese. Its English accessibility name starts with
**Not connected?**. Review Notes must describe the visible card; the tests match
the localized **See a demo / 看看演示** action. The extra `00-demo-entry.png` captures
verify the reviewer entry and are evidence, not additional store screenshots.
