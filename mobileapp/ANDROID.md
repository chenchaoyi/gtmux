# Android

[中文](ANDROID.zh.md)

Android uses the same React Native screens, API client, pairing store, Chat, Terminal,
HQ, Trace, Diff and code readers as iPhone/iPad. Kotlin supplies document transport,
WebView/PDF rendering, system sharing, haptics, hardware-keyboard events, FCM and the
home-screen widget. The Go server remains the authority for session and file access.

## Install and connect

A signed APK installs directly on Android 7.0/API 24 or later, on arm64 or x86_64.
AAB is the Google Play upload format; it is not a directly installable file.
Open the private installation link on Android, download the APK and allow that browser
as an installation source if prompted. Keep the previous app and pairing data when
updating. A signature conflict needs investigation, not an uninstall or disabling Play
Protect.

Pair using the existing QR, `gtmux://` link or address/token form. LAN HTTP remains
supported for an address you paired; remote HTTPS uses normal certificate validation.
Camera permission is requested only when scanning. Denying it leaves manual pairing
available. Guest permissions, input guards and local-file authorization are unchanged.

## Platform behavior

- Phone and tablet use the existing responsive shell. Android Back closes a sheet or
  returns through navigation; hardware Ctrl shortcuts reuse the shared keymap. Plain
  arrow/Return keys remain available to text fields and the IME.
- Documents download into a private, temporary directory, with session/home matching,
  size and SHA-256 checks. HTML is static until explicit consent for that file's
  scripts; page network, file access, frames and native JavaScript bridges stay blocked.
  Local images resolve through the same parent-document API as iOS. Shared files use a
  narrowly granted `content://` URI. Closing the preview removes its private cache. A shared copy remains in a separate, narrowly exposed cache for 24 hours (256 MiB total budget), so closing the reader does not break another app’s access. Text/HTML rendering is bounded to 8 MiB; the full verified original remains shareable up to 30 MiB.
- The HQ copy is downloaded atomically and can be exported through Android's share
  sheet. Android system/cloud backup is disabled; keeping a local copy does not imply
  a cloud backup. Exports are unencrypted and need secure storage.
- The usage widget holds a credential-free **last-seen** snapshot from the selected
  owner connection. The foreground app refreshes it; opening the widget opens Usage.
  Changing/removing the connection or viewing a guest clears the previous snapshot.
- Background push is optional FCM. The release needs Firebase client configuration,
  the relay's separate service account, Google Play services and notification permission.
  Settings names missing configuration/services/permission. The rest of the app works
  without Google services. A notification opens the current session; Android does not
  send approval numbers from a potentially stale notification. Use its current prompt.
  FCM delivery, OEM background policies and manufacturer push are separate device tests.

## Build and automate

Use Node 22.12+, JDK 17, SDK/Build Tools 36 and NDK 27.1.12297006. The wrapper is
Gradle 9.3.1 with a pinned distribution checksum. Reuse an existing SDK; a second IDE
installation is unnecessary.

```sh
cd mobileapp
npm ci
npm run check -- --runInBand
cd android
./gradlew :app:assembleQa :app:testQaUnitTest :app:lintQa --no-daemon
```

`com.gtmux.app.qa` bundles JS and runs without Metro. Its private fixture bridge is
registered only in QA/debug builds. Production `com.gtmux.app` has no fixture module.
Release refuses missing signing credentials or a missing exact source SHA. The public
debug keystore is exclusively for QA/debug; it is never a release fallback.

Run only on an owned AVD named `gtmux-qa-*` or `gtmux-ci*`:

```sh
export APPIUM_HOME="$(mktemp -d)"
npx appium driver install --source=local "$PWD/node_modules/appium-uiautomator2-driver"
export ANDROID_SERIAL=emulator-5554
npm run test:e2e:android
```

The runner rejects physical devices and production packages, starts its own Appium
and synthetic server, records actual writes, saves failure screenshots and closes only
its own processes. A green unit/build job is separate from emulator taps, TalkBack,
physical-device/OEM behavior, live FCM and store acceptance. iOS tests keep their current
entry point; Android does not invoke their simulator setup or reclaim routine.

## Signed delivery

**Android private delivery build** accepts an exact reviewed main commit whose latest
main CI and Android emulator job passed. CI builds production-mode unsigned APK/AAB,
checks bundled JS/native bytes, both 64-bit ABIs, source identity and 16 KB ELF
alignment, then uploads only an age-encrypted result. It receives a public result
recipient; application/upload private keys remain on the operator's machine.

Download the exact successful run artifact and independently verify its SHA/digest,
source receipt and each file hash. In a clean local output directory, use
`scripts/android-delivery.py sign --input UNSIGNED --output SIGNED --source SHA --sdk SDK`
with `GTMUX_ANDROID_KEYSTORE`, `GTMUX_ANDROID_STORE_PASSWORD`, `GTMUX_ANDROID_KEY_ALIAS`
and `GTMUX_ANDROID_KEY_PASSWORD` in the private environment. Then run `verify` on the
signed APK/AAB against the pinned application certificate. The verifier checks the
signature, source/version, JS/native equivalence, both ABIs, 16 KB ELF/ZIP alignment
and Google bundle validation. Neither a build nor a download is an installation receipt.
R8 mappings, JS source maps, native symbols and source archive travel with the encrypted
build, so signing does not lose crash recovery. See [store readiness](../docs/android/store.md).

Optional public Firebase client repository variables use `ANDROID_FIREBASE_APP_ID`,
`ANDROID_FIREBASE_SENDER_ID`, `ANDROID_FIREBASE_API_KEY`, `ANDROID_FIREBASE_PROJECT_ID`.
The relay's separate service-account secret never belongs in a client package or build.
Local trusted builds can also use `build` without `--unsigned` when signing inputs are
already present; absence of credentials never falls back to the QA signer.

Version name follows `src/version.ts`. Build ordinals are 1–999; versionCode is
`major × 100000000 + minor × 1000000 + patch × 1000 + ordinal` (minor ≤99, patch ≤999,
Play's 2100000000 ceiling). Published version codes are never reused for a different
package. Direct updates must retain their application-signing identity; future Play
App Signing must deliberately preserve that identity or use a separate application ID.
A separate Play upload key is not the application's update identity.

Keep intermediate builds and verification copies in the task/build cache. Preserve only
unique encrypted signing/symbol/source recovery and small evidence outside application
runtime state, with an owner, size budget and review date. Artifact expiry is not an
offsite recovery policy; do not delete the only signing or recovery material.

Android image Paste reads only on an explicit tap, through a size/pixel-bounded private temporary file. Markup reuses shared drawing/export, with Android pinch/pan handling and full-pixel JPEG capture. Automation uses Node 24.12+ and owned emulators; orientation is verified from actual screen aspect.
