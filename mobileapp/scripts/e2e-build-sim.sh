#!/usr/bin/env bash
# Build gtmux.app for the iOS simulator and (re)install it FRESH on the booted
# sim, so the Appium e2e suite (noReset:true) starts from a clean Keychain — the
# app opens on the connection page. Run this before `npm run test:e2e`, and again
# after any source change (the e2e session does not rebuild).
#
# Targets the currently-BOOTED simulator by UDID (robust against duplicate device
# names across runtimes). Boot one first: `xcrun simctl boot "iPhone 17 Pro"`.
set -euo pipefail

APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"   # mobileapp/
DD="${GTMUX_E2E_DERIVED:-$APP_DIR/ios/build-e2e}"
BUNDLE="com.gtmux.app"

UDID="${GTMUX_E2E_UDID:-$(xcrun simctl list devices booted | grep -oE '[0-9A-Fa-f-]{36}' | head -1)}"
[ -n "$UDID" ] || { echo "[e2e-build] no booted simulator — run: xcrun simctl boot 'iPhone 17 Pro'"; exit 1; }
echo "[e2e-build] target sim: $UDID"

echo "[e2e-build] building Release for the simulator…"
arch -arm64 xcodebuild \
  -workspace "$APP_DIR/ios/GtmuxMobile.xcworkspace" -scheme GtmuxMobile \
  -configuration Release -sdk iphonesimulator \
  -destination "platform=iOS Simulator,id=$UDID" \
  -derivedDataPath "$DD" CODE_SIGNING_ALLOWED=NO build \
  > "$APP_DIR/ios/build-e2e.log" 2>&1 \
  || { echo "[e2e-build] BUILD FAILED — tail of log:"; tail -25 "$APP_DIR/ios/build-e2e.log"; exit 1; }

APP="$DD/Build/Products/Release-iphonesimulator/gtmux.app"
[ -d "$APP" ] || { echo "[e2e-build] no app at $APP"; exit 1; }

echo "[e2e-build] reinstalling fresh (clean Keychain)…"
xcrun simctl terminate "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl uninstall "$UDID" "$BUNDLE" 2>/dev/null || true
xcrun simctl install "$UDID" "$APP"
echo "[e2e-build] installed $BUNDLE on $UDID."
