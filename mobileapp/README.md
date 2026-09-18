# gtmux mobile app

The phone surface for gtmux — monitor your tmux coding agents remotely (over a
VPN/tunnel) and get lock-screen push when one needs you or finishes.

This is a committed React Native 0.86 / TypeScript app. The build artifact and
display name are **gtmux** (`gtmux.app`, bundle `com.gtmux.app`); the internal
Xcode scheme/target and the JS `AppRegistry` module are still `GtmuxMobile` (that
pair must stay matched). It's a pure consumer of `gtmux serve` (see
`api/contract.md`) and mirrors the macOS menu-bar app's status language
(`docs/design/DESIGN.md`, `docs/design/MOBILE.md`, `macapp/Sources/GtmuxBar/`) so
the CLI, menu-bar, and phone look like one product.

Build & run on a Mac (needs Xcode + CocoaPods; design/setup detail in
[`SPEC.md`](./SPEC.md)):

```sh
npm install
cd ios && arch -arm64 pod install && cd ..
# simulator:
npx react-native run-ios
# signed device build (Release bundles the JS, runs untethered):
xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
  -configuration Release -destination 'generic/platform=iOS' \
  -derivedDataPath ios/build -allowProvisioningUpdates DEVELOPMENT_TEAM=<team> \
  APS_ENVIRONMENT=development build
ideviceinstaller -u "$(idevice_id -l)" install \
  ios/build/Build/Products/Release-iphoneos/gtmux.app
# then pair with a running `gtmux serve` (scan the QR, or enter host + token)
```

Two choices above keep this working on a locked phone. The build targets
`generic/platform=iOS` rather than the device's udid, because a udid destination waits
for the device to become ready, which mounts the developer disk image, which a locked
phone refuses. And the install goes through `ideviceinstaller` (`brew install
ideviceinstaller`), which uses the system's installation service and needs no disk
image; `xcrun devicectl device install app` only works while the image happens to be
mounted, which stops being true after a reboot or an iOS update.

`APS_ENVIRONMENT=development` matches the development signing to sandbox push. Leave it
off for an App Store archive, which defaults to production.

Don't `rm -rf ios/build` without re-running `pod install` — it wipes the RN
codegen and the next compile fails on missing `*-generated.mm`.

Scope: live colored terminal view (`/api/pane`), focus, terminal **write**
(`/api/send`), file/image upload + image-paste markup, a draggable nav keypad,
full-screen per session, official agent icons (`/api/icon`), and lock-screen
**push** with per-kind filtering. The bearer token gates writes — a leaked token
is RCE on the Mac, so keep the tunnel deliberate.
