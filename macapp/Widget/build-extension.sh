#!/usr/bin/env bash
# Xcode builds both bundles so App Intents metadata is extracted for each host.
# Signing belongs to build.sh; this helper also runs unsigned in PR CI.
set -euo pipefail
cd "$(dirname "$0")/.."
widget_archs="$(uname -m)"
if [ "${GTMUX_UNIVERSAL:-0}" = 1 ]; then widget_archs="arm64 x86_64"; fi
mkdir -p build
python3 - <<'PLIST'
import os, plistlib
from pathlib import Path
text = Path("Info.plist").read_text()
for key, value in {"__VERSION__": os.environ.get("GTMUX_WIDGET_VERSION", "0.0.0"),
                   "__BUILD_VERSION__": os.environ.get("GTMUX_WIDGET_VERSION", "0.0.0"),
                   "__SIGNING_TEAM__": os.environ.get("GTMUX_WIDGET_TEAM", ""),
                   "__USAGE_GROUP__": os.environ.get("GTMUX_WIDGET_GROUP", "")}.items():
    text = text.replace(key, value)
info = plistlib.loads(text.encode())
info["CFBundleIdentifier"] = "$(PRODUCT_BUNDLE_IDENTIFIER)"
text = plistlib.dumps(info).decode()
Path("build/Parent-Info.plist").write_text(text)
PLIST
xcodebuild -project GtmuxWidgets.xcodeproj -scheme Gtmux \
  -configuration Release -derivedDataPath build/widget \
  GTMUX_HOST_PREVIEW_CONDITIONS= CODE_SIGNING_ALLOWED=NO ONLY_ACTIVE_ARCH=NO "ARCHS=$widget_archs" \
  "MARKETING_VERSION=${GTMUX_WIDGET_VERSION:-0.0.0}" \
  "CURRENT_PROJECT_VERSION=${GTMUX_WIDGET_VERSION:-0.0.0}" \
  "GTMUX_SIGNING_TEAM=${GTMUX_WIDGET_TEAM:-}" "GTMUX_USAGE_GROUP=${GTMUX_WIDGET_GROUP:-}" build
host_product="build/widget/Build/Products/Release/Gtmux.app"
widget_product="$host_product/Contents/PlugIns/GtmuxUsageWidget.appex"
test -f "$host_product/Contents/MacOS/GtmuxBar"
python3 Widget/verify_bundle.py "$host_product"
