#!/usr/bin/env bash
# Release the macOS app from your Mac: build a Developer ID-signed + NOTARIZED
# Gtmux.app, upload it to the GitHub release for the tag, and update the Homebrew
# cask. For the "notarize locally" path (no GitHub signing secrets) — the notary
# credentials live in a keychain profile. One-time setup + how-to: docs/release-signing.md.
#
#   make app-release                 # uses the latest tag
#   macapp/release.sh v0.12.40       # or an explicit tag
#
# Prereqs: a "Developer ID Application" cert in your keychain, a notarytool keychain
# profile (default name `gtmux-notary`, override with GTMUX_NOTARY_PROFILE), `gh`
# logged in, and the tag already pushed (so goreleaser created the release).
set -euo pipefail
cd "$(dirname "$0")/.."   # repo root

TAG="${1:-$(git describe --tags --abbrev=0)}"
VERSION="${TAG#v}"
PROFILE="${GTMUX_NOTARY_PROFILE:-gtmux-notary}"

# Auto-derive the signing identity (no need to set GTMUX_SIGN_ID).
SIGN_ID="${GTMUX_SIGN_ID:-$(security find-identity -v -p codesigning | grep -o '"Developer ID Application:[^"]*"' | head -1 | tr -d '"')}"
[ -n "$SIGN_ID" ] || { echo "no 'Developer ID Application' identity in your keychain — see docs/release-signing.md" >&2; exit 1; }

echo "==> releasing $TAG"
echo "    identity: $SIGN_ID"
echo "    notary profile: $PROFILE"
gh release view "$TAG" >/dev/null 2>&1 || { echo "release $TAG not found — push the tag first (git push origin $TAG)" >&2; exit 1; }

# 1. Build (Developer ID sign + notarize + staple, via build.sh).
GTMUX_UNIVERSAL=1 GTMUX_VERSION="$TAG" GTMUX_SIGN_ID="$SIGN_ID" GTMUX_NOTARY_PROFILE="$PROFILE" macapp/build.sh

# 2. Zip the stapled bundle.
ZIP="Gtmux-${VERSION}-macos.zip"
rm -f "$ZIP"
ditto -c -k --keepParent macapp/build/Gtmux.app "$ZIP"
SHA="$(shasum -a 256 "$ZIP" | awk '{print $1}')"
echo "==> $ZIP  sha256=$SHA"

# 3. Upload to the release (clobber any ad-hoc build CI may have attached).
gh release upload "$TAG" "$ZIP" --clobber
echo "==> uploaded to release $TAG"

# 4. Update the Homebrew cask (gtmux-app) in the tap.
TAP="$(mktemp -d)"
gh repo clone chenchaoyi/homebrew-tap "$TAP" -- --depth 1 >/dev/null 2>&1
mkdir -p "$TAP/Casks"
cat > "$TAP/Casks/gtmux-app.rb" <<CASK
# Published by gtmux's release flow. DO NOT EDIT.
cask "gtmux-app" do
  version "${VERSION}"
  sha256 "${SHA}"

  url "https://github.com/chenchaoyi/gtmux/releases/download/v#{version}/Gtmux-#{version}-macos.zip"
  name "Gtmux"
  desc "Menu-bar companion for the gtmux session overview"
  homepage "https://github.com/chenchaoyi/gtmux"

  depends_on macos: ">= :ventura"

  app "Gtmux.app"

  zap trash: [
    "~/Library/Preferences/com.gtmux.menubar.plist",
  ]
end
CASK
(
  cd "$TAP"
  git add Casks/gtmux-app.rb
  if git diff --cached --quiet; then
    echo "==> cask already at ${VERSION}"
  else
    git commit -q -m "gtmux-app ${VERSION}"
    git push -q
    echo "==> cask gtmux-app → ${VERSION}"
  fi
)
rm -rf "$TAP" "$ZIP"
echo "DONE — teammates: brew install --cask chenchaoyi/tap/gtmux-app  (opens directly, notarized)"
