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
LDFLAGS="-s -w -X github.com/chenchaoyi/gtmux/internal/app.Version=${VERSION}"

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
sed "s/__VERSION__/${VERSION}/g" Info.plist > "$BUNDLE/Contents/Info.plist"
cp "$SWIFT_BIN" "$BUNDLE/Contents/MacOS/${APP_BIN}"
cp build/gtmux "$BUNDLE/Contents/MacOS/gtmux"
chmod +x "$BUNDLE/Contents/MacOS/"*

echo "==> ad-hoc code signing"
codesign --force --deep --sign - "$BUNDLE"

echo
echo "Built $(pwd)/$BUNDLE (version ${VERSION})"
