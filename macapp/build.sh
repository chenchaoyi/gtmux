#!/usr/bin/env bash
# Build Gtmux.app — the native macOS menu-bar app — from Swift source, bundling
# the cgo-free gtmux CLI alongside it (version-matched, no $PATH reliance).
#
#   ./build.sh                 # native-arch build → build/Gtmux.app
#   GTMUX_UNIVERSAL=1 ./build.sh   # universal (arm64 + x86_64) — used by CI
#   GTMUX_VERSION=v1.2.3 ./build.sh
#
# Requires: Swift 5.9+ (Xcode CLT) and the Go toolchain (for the bundled CLI).
set -euo pipefail
cd "$(dirname "$0")"
REPO_ROOT="$(cd .. && pwd)"

BUNDLE="build/Gtmux.app"
APP_BIN="GtmuxBar"
VERSION="${GTMUX_VERSION:-$(cd "$REPO_ROOT" && git describe --tags --always --dirty 2>/dev/null || echo dev)}"
# GTMUX_TUNNEL_REG bakes the hosted-tunnel gate into the BUNDLED CLI too, so the
# menu-bar "remote access" toggle can drive hosted `gtmux tunnel` (empty → off).
LDFLAGS="-s -w -X github.com/chenchaoyi/gtmux/internal/app.Version=${VERSION} -X github.com/chenchaoyi/gtmux/internal/app.TunnelRegSecret=${GTMUX_TUNNEL_REG:-} -X github.com/chenchaoyi/gtmux/internal/app.RelayToken=${GTMUX_RELAY_TOKEN:-}"

echo "==> swift build (release) — version ${VERSION}"
if [ "${GTMUX_UNIVERSAL:-}" = "1" ]; then
  swift build -c release --arch arm64 --arch x86_64
  SWIFT_BIN=".build/apple/Products/Release/${APP_BIN}"
else
  swift build -c release
  SWIFT_BIN=".build/release/${APP_BIN}"
fi
[ -f "$SWIFT_BIN" ] || { echo "swift build failed: missing $SWIFT_BIN" >&2; exit 1; }

echo "==> go build gtmux CLI (cgo-free) to bundle"
mkdir -p build
if [ "${GTMUX_UNIVERSAL:-}" = "1" ]; then
  ( cd "$REPO_ROOT"
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o "macapp/build/gtmux-arm64" ./cmd/gtmux
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "macapp/build/gtmux-amd64" ./cmd/gtmux )
  lipo -create -output build/gtmux build/gtmux-arm64 build/gtmux-amd64
  rm -f build/gtmux-arm64 build/gtmux-amd64
else
  ( cd "$REPO_ROOT" && CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "macapp/build/gtmux" ./cmd/gtmux )
fi

echo "==> assembling $BUNDLE"
rm -rf "$BUNDLE"
mkdir -p "$BUNDLE/Contents/MacOS"
# `|` delimiter: VERSION may be a branch name with '/' (workflow_dispatch builds).
sed "s|__VERSION__|${VERSION}|g" Info.plist > "$BUNDLE/Contents/Info.plist"
cp "$SWIFT_BIN" "$BUNDLE/Contents/MacOS/${APP_BIN}"
cp build/gtmux "$BUNDLE/Contents/MacOS/gtmux"
chmod +x "$BUNDLE/Contents/MacOS/"*
# App icon (the gtmux pane-grid logo) — Info.plist points CFBundleIconFile at it.
if [ -f AppIcon.icns ]; then
  mkdir -p "$BUNDLE/Contents/Resources"
  cp AppIcon.icns "$BUNDLE/Contents/Resources/AppIcon.icns"
fi

# Code signing. Set GTMUX_SIGN_ID to a "Developer ID Application: …" identity
# for a STABLE signature (TCC permissions then persist across updates instead of
# re-prompting every reinstall — the ad-hoc fallback changes identity each build)
# and Hardened Runtime (required for notarization). Falls back to ad-hoc.
# Sign nested code (the bundled CLI) BEFORE the outer bundle; avoid --deep.
SIGN_ID="${GTMUX_SIGN_ID:-}"
if [ -n "$SIGN_ID" ]; then
  echo "==> code signing (Developer ID, hardened runtime): $SIGN_ID"
  codesign --force --options runtime --timestamp --sign "$SIGN_ID" "$BUNDLE/Contents/MacOS/gtmux"
  codesign --force --options runtime --timestamp --sign "$SIGN_ID" "$BUNDLE/Contents/MacOS/${APP_BIN}"
  codesign --force --options runtime --timestamp --sign "$SIGN_ID" "$BUNDLE"
  codesign --verify --strict --verbose=2 "$BUNDLE" || { echo "codesign verify failed" >&2; exit 1; }
  echo "   signed. Notarize the zipped bundle next, e.g.:"
  echo "     ditto -c -k --keepParent \"$BUNDLE\" Gtmux.zip"
  echo "     xcrun notarytool submit Gtmux.zip --keychain-profile \"\$GTMUX_NOTARY_PROFILE\" --wait"
  echo "     xcrun stapler staple \"$BUNDLE\""
else
  echo "==> ad-hoc code signing (set GTMUX_SIGN_ID='Developer ID Application: …' for a stable signature)"
  codesign --force --deep --sign - "$BUNDLE"
fi

echo
echo "Built $(pwd)/$BUNDLE (version ${VERSION})"
