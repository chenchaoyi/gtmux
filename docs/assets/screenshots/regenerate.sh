#!/usr/bin/env bash
# Regenerate every user-facing screenshot with GENERIC data (no personal session
# names, paths, server name, or cost). See ./README.md for prerequisites.
#
#   Mobile (real captures)  — iOS simulator + a throwaway mock serve → the app's
#                             own GTMUX_SHOTS e2e harness drives radar/detail/servers.
#   Hero  (rendered)        — self-contained HTML rendered by headless Chrome.
#
# Outputs (committed): docs/assets/{surface-cli,surface-menubar,surface-mobile,
#                      screenshot-detail,screenshot-servers}.png
#
# Usage:
#   bash docs/assets/screenshots/regenerate.sh            # everything
#   GTMUX_SKIP_CAPTURE=1 bash …/regenerate.sh             # heroes only (reuse last sim shots)
#   GTMUX_SHOTS_BUILD=1  bash …/regenerate.sh             # rebuild+install the sim app first
# Env: GTMUX_E2E_UDID (booted sim udid; auto-detected), GTMUX_E2E_OS (default 26.4).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
ASSETS="$REPO/docs/assets"
APP="$REPO/mobileapp"
SHOTS="$APP/.e2e-artifacts/shots"

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$CHROME" ] || { echo "✗ Google Chrome not found (needed to render the hero images)"; exit 1; }
command -v sips >/dev/null || { echo "✗ sips not found (macOS only)"; exit 1; }

# ── 1. Mobile captures (unless skipped) ──────────────────────────────────────
if [ "${GTMUX_SKIP_CAPTURE:-0}" != "1" ]; then
  UDID="${GTMUX_E2E_UDID:-$(xcrun simctl list devices booted | grep -oE '[0-9A-F-]{36}' | head -1 || true)}"
  [ -n "$UDID" ] || { echo "✗ no booted simulator — boot one (see README) or set GTMUX_E2E_UDID"; exit 1; }
  echo "▸ simulator: $UDID"

  echo "▸ starting mock serve on :8799"
  node "$HERE/mock-serve.js" &
  MOCK=$!
  trap 'kill "$MOCK" 2>/dev/null || true' EXIT
  for _ in $(seq 1 20); do curl -sf -m2 http://127.0.0.1:8799/api/health >/dev/null 2>&1 && break; sleep 0.3; done
  curl -sf -m2 http://127.0.0.1:8799/api/health >/dev/null || { echo "✗ mock serve did not come up"; exit 1; }

  cd "$APP"
  if [ "${GTMUX_SHOTS_BUILD:-0}" = "1" ]; then
    echo "▸ building + installing the app on the sim…"
    GTMUX_E2E_UDID="$UDID" npm run e2e:build
  fi

  echo "▸ capturing radar / detail / servers from the simulator…"
  rm -rf "$SHOTS"
  GTMUX_SHOTS=1 \
  GTMUX_E2E_URL=http://127.0.0.1:8799 \
  GTMUX_E2E_TOKEN=demo-token \
  GTMUX_SHOTS_NAME="${GTMUX_SHOTS_NAME:-demo-mac}" \
  GTMUX_E2E_OS="${GTMUX_E2E_OS:-26.4}" \
  GTMUX_E2E_UDID="$UDID" \
    npx jest --config e2e/jest.config.e2e.js --runInBand e2e/__tests__/screenshots.test.ts
  kill "$MOCK" 2>/dev/null || true; trap - EXIT
fi

[ -f "$SHOTS/radar.png" ] || { echo "✗ no captured shots in $SHOTS — run without GTMUX_SKIP_CAPTURE first"; exit 1; }

# phone.md raw images (276×600, same aspect as the sim capture).
cp "$SHOTS/detail.png"  "$ASSETS/screenshot-detail.png"
cp "$SHOTS/servers.png" "$ASSETS/screenshot-servers.png"
sips --resampleWidth 276 "$ASSETS/screenshot-detail.png"  >/dev/null
sips --resampleWidth 276 "$ASSETS/screenshot-servers.png" >/dev/null
echo "▸ wrote screenshot-detail.png / screenshot-servers.png (276×600)"

# ── 2. README hero images (rendered HTML → PNG) ──────────────────────────────
VERSION="$(git -C "$REPO" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo dev)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# mobile hero frames the fresh radar capture; menubar hero gets the live version.
cp "$SHOTS/radar.png" "$TMP/radar-hero.png"; sips --resampleWidth 560 "$TMP/radar-hero.png" >/dev/null
cp "$HERE/cli-hero.html" "$TMP/cli-hero.html"
cp "$HERE/mobile-hero.html" "$TMP/mobile-hero.html"
sed "s/__VERSION__/$VERSION/" "$HERE/menubar-hero.html" > "$TMP/menubar-hero.html"

render() { # <html-basename> <out-name>
  "$CHROME" --headless --disable-gpu --hide-scrollbars --allow-file-access-from-files \
    --force-device-scale-factor=2 --window-size=612,760 \
    --screenshot="$TMP/$2.png" "file://$TMP/$1" >/dev/null 2>&1
  cp "$TMP/$2.png" "$ASSETS/$2.png"
  sips --resampleWidth 612 "$ASSETS/$2.png" >/dev/null   # 1224×1520 → 612×760
}
render cli-hero.html     surface-cli
render menubar-hero.html surface-menubar
render mobile-hero.html  surface-mobile
echo "▸ wrote surface-cli/menubar/mobile.png (612×760, version $VERSION)"

echo "✓ done — review: git status docs/assets/"
