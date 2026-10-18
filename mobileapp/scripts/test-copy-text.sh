#!/usr/bin/env bash
# Use a disposable booted Simulator; never pass a physical device or shared app.
set -euo pipefail
sim="${1:?Pass the UUID of a disposable booted iOS Simulator}"
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/gtmux-copy-text.XXXXXX")
bundle_id=dev.gtmux.copy-text-checks
cleanup() {
  xcrun simctl terminate "$sim" "$bundle_id" >/dev/null 2>&1 || true
  xcrun simctl uninstall "$sim" "$bundle_id" >/dev/null 2>&1 || true
  /bin/rm -rf "$work"
}
trap cleanup EXIT
sdk=$(xcrun --sdk iphonesimulator --show-sdk-path)
mkdir -p "$work/CopyTextChecks.app"
SDKROOT="$sdk" xcrun --sdk iphonesimulator swiftc -sdk "$sdk" -target "$(uname -m)-apple-ios16.0-simulator" \
  -module-cache-path "$work/cache" -framework UIKit \
  "$root/ios/CopyText/CopyTextView.swift" "$root/ios/CopyTextTests/Runner.swift" \
  -o "$work/CopyTextChecks.app/CopyTextChecks"
/usr/libexec/PlistBuddy -c "Add :CFBundleIdentifier string $bundle_id" \
  -c 'Add :CFBundleExecutable string CopyTextChecks' -c 'Add :CFBundlePackageType string APPL' \
  -c 'Add :CFBundleVersion string 1' -c 'Add :CFBundleShortVersionString string 1.0' \
  -c 'Add :MinimumOSVersion string 16.0' "$work/CopyTextChecks.app/Info.plist"
codesign --force --sign - "$work/CopyTextChecks.app" >/dev/null
xcrun simctl install "$sim" "$work/CopyTextChecks.app"
xcrun simctl launch --console "$sim" "$bundle_id" | tee "$work/test.log"
grep -q '^COPY_TEXT_CHECKS_PASSED:' "$work/test.log"
