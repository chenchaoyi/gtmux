# Capture App Store images in Actions

Run **App Store screenshots (maintainers)** (`ios-store-assets.yml`) from `main`
with a published stable `release_tag`. It verifies the release and successful main CI.
No signing, publisher or ASC credential is needed. [中文](assets-ci.zh.md)

The workflow separates three jobs:

1. **build** creates the exact released app once. Its public `store-simulator-<run>-<attempt>`
   artifact contains an unsigned Simulator app and source/toolchain/content manifest.
2. **capture** installs that app without Pods or a native rebuild. Dedicated iPhone
   17 Pro Max and iPad Pro 13-inch simulators capture English/Chinese. Each completed
   group is hashed and saved in `store-checkpoint-<run>-<attempt>`, including on failure.
   Only newly created simulator IDs are shut down/deleted.
3. **render** validates all four groups and generates 22 screenshots plus two creative
   images with current reviewed scripts/captions. Widget rendering uses released source.
   `appstore-assets-<tag>-<run>-<attempt>` includes raw images and separate capture/render provenance.

```sh
# Fresh capture
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z
# Resume a completed failed/successful run, reusing its build and valid groups
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z -f reuse_run=RUN_ID
# Caption/layout update only: no native build, Appium or simulator
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z \
  -f reuse_run=RUN_ID -f stage=render-only
```

Reuse requires a completed manual run of this workflow on this repository's main.
Select a capture run to reuse its build; a render-only run forwards raw checkpoints
but has no build artifact. Source, Xcode, architecture and app hashes must match.
Mismatched build artifacts stop with guidance to build fresh. Changed capture inputs
invalidate group completion; corrupted/missing recorded files fail rather than being
silently trusted. Render-only requires four complete matching groups. Artifacts expire
after **7 days**; missing required/expired artifacts need a fresh capture. Keep accepted
raws/manifests privately for longer retention. A partial checkpoint is never a final
asset manifest. Current caption edits can reframe old verified raw images without
changing the released app or losing original capture provenance.

Download final assets, inspect every image at original size, and copy screenshots
and creative trees into the corresponding `mobileapp/fastlane/` folders. Retain the
manifest and follow [preparation/readback](preparation.md). The lock-screen image
remains a widget rendering, not real push acceptance. This workflow does not write to
ASC or submit. It does not establish real-device, VoiceOver, APNs or Live Activity acceptance.
Public output must never contain device IPAs, profiles, credentials or real session data.

The runner registers the XCUITest driver from npm at the version in the release's
lockfile. Do not register its installed `node_modules` folder with `--source=local`:
Appium treats that as a source checkout and runs `prepare`, but the published
package does not include the TypeScript project needed for that rebuild (TS5083).

Binary preparation is separate. See [reusing a verified release IPA](reuse-ipa.md)
for the additional signing and validation required when local disk space is limited.

## Capture harness and diagnostics

The app is built from the published tag. The reviewed workflow supplies only the
two capture tests, their demo selector, simulator capabilities and session setup; it never
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

Hosted capture compiles WebDriverAgent in a separate process before Appium starts,
then uses `usePrebuiltWDA`. Do not use `prebuildWDA` on WDA 15.0.0: its build-only
exit flag remains set, so the next launch skips readiness and immediately refuses
the session. Run 38913607022 confirmed a successful three-minute build followed by
this failure; the [versioned source](https://github.com/appium/WebDriverAgent/blob/v15.0.0/lib/xcodebuild.ts)
shows the retained flag. Nested and hoisted driver layouts are both resolved from
this run's Appium home. Compiler output is kept in `wda-build-<device>.log`.
Run 38907980603 repeatedly terminated its cold xcodebuild after readiness timeouts;
the missing Xcode output prevented a more specific diagnosis. The hosted-only
startup now records that output, sets `wdaLaunchTimeout=240000`, and
uses one connection bounded to 15 minutes instead of overlapping automatic retries.
Ordinary local e2e defaults stay unchanged. A timeout remains a failed capture;
it does not prove a simulator, app or screenshot passed.

The hosted multi-screen capture has a five-minute test budget; individual element
waits retain their short bounds and ordinary local tests keep two minutes. Run
38920162585 reached six phone screens before the old overall two-minute budget
expired. The usage sheet closes through its visible Done button, without waiting
for an absent detail Back button. Each saved raw image logs its name and time.
