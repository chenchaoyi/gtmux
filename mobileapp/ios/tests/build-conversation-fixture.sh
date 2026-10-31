#!/usr/bin/env bash
# Compile a UIKit-only acceptance app without CocoaPods, a server or real pairing.
set -euo pipefail
OUT="${1:?Pass a private build directory}"
IOS="$(cd "$(dirname "$0")/.." && pwd)"
APP="$OUT/ConversationFixture.app"
mkdir -p "$APP"
cat > "$APP/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>ConversationFixture</string><key>CFBundleIdentifier</key><string>com.gtmux.conversation-fixture</string><key>CFBundleName</key><string>Conversation Fixture</string><key>CFBundleVersion</key><string>1</string><key>CFBundleShortVersionString</key><string>1.0</string><key>CFBundlePackageType</key><string>APPL</string><key>MinimumOSVersion</key><string>15.1</string><key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array><key>UIRequiresFullScreen</key><true/><key>UILaunchScreen</key><dict/><key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict><key>UISupportedInterfaceOrientations</key><array><string>UIInterfaceOrientationPortrait</string><string>UIInterfaceOrientationLandscapeLeft</string><string>UIInterfaceOrientationLandscapeRight</string></array></dict></plist>
PLIST
xcrun --sdk iphonesimulator swiftc -module-cache-path "$OUT/ModuleCache" -sdk "$(xcrun --sdk iphonesimulator --show-sdk-path)" -target arm64-apple-ios15.1-simulator -o "$APP/ConversationFixture" "$IOS/tests/ConversationDocumentsFixture.swift" "$IOS/Documents/DocumentTransport.swift" "$IOS/Documents/DocumentHTML.swift" "$IOS/Documents/DocumentMarkdown.swift" "$IOS/Documents/DocumentResources.swift" "$IOS/Documents/DocumentStatusView.swift" "$IOS/Documents/DocumentPreviewController.swift" "$IOS/ConversationText/ConversationTextView.swift"
codesign --force --sign - "$APP"
printf '%s\n' "$APP"
