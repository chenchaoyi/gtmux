#!/usr/bin/env bash
# Regenerate every user-facing screenshot with GENERIC data (no personal session
# names, paths, server name, or cost). See ./README.md for prerequisites.
#
#   Phone doc shots (real captures) — iOS simulator + a throwaway mock serve; the app's
#                                     own GTMUX_SHOTS e2e harness drives detail/servers.
#   README artwork (rendered)       — HTML templates here, filled with the App Store
#                                     captures and rendered by headless Chrome.
#
# Outputs (committed): docs/assets/{screenshot-detail,screenshot-servers}.png and
#                      docs/assets/readme-{hero,hero-dark,screens,screens-dark}.jpg
#
# Usage:
#   bash docs/assets/screenshots/regenerate.sh            # everything
#   GTMUX_ONLY=readme    bash …/regenerate.sh             # README artwork only (no simulator)
#   GTMUX_SKIP_CAPTURE=1 bash …/regenerate.sh             # reuse the last simulator shots
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

# ── 1. Phone doc captures (unless skipped) ──────────────────────────────────
if [ "${GTMUX_ONLY:-}" != "readme" ]; then
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

fi

# ── 2. README artwork (rendered HTML → JPEG) ─────────────────────────────────
# The hero (terminal, iPad, iPhone) and a strip of three phone screens, each in light
# and dark; the README picks one with <picture>. Built from the App Store captures,
# which the appstore-shots e2e writes (docs/appstore/submit.md).
STORE="$APP/.e2e-artifacts/appstore"
for f in en/01-radar.png en/02-lockscreen.png en/02-terminal-approval.png ipad-en/01-split.png; do
  [ -f "$STORE/$f" ] || { echo "✗ missing $STORE/$f — run the appstore-shots e2e first (docs/appstore/submit.md)"; exit 1; }
done
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
art() { # <template> <out-name> <width> <height>
  sed -e "s#__IPAD__#$STORE/ipad-en/01-split.png#" -e "s#__LOCK__#$STORE/en/02-lockscreen.png#" \
      -e "s#__RADAR__#$STORE/en/01-radar.png#" -e "s#__REPLY__#$STORE/en/02-terminal-approval.png#" \
      "$HERE/$1" > "$TMP/$1"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
    --force-device-scale-factor=2 --window-size="$3,$4" \
    --screenshot="$TMP/$2.png" "file://$TMP/$1" >/dev/null 2>&1
  sips -s format jpeg -s formatOptions 86 "$TMP/$2.png" --out "$ASSETS/$2.jpg" >/dev/null
}
art readme-hero-light.html    readme-hero         1280 640
art readme-hero-dark.html     readme-hero-dark    1280 640
art readme-screens-light.html readme-screens      1280 720
art readme-screens-dark.html  readme-screens-dark 1280 720
echo "▸ wrote readme-hero(-dark).jpg 2560×1280 and readme-screens(-dark).jpg 2560×1440"

echo "✓ done — review: git status docs/assets/"
