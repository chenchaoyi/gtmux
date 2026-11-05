#!/bin/bash
# Isolated hosted runner only. Both versions use this checkout's identical test harness
# and the same debug-only instrumentation; neither uses signing credentials.
set -euo pipefail
: "${RUNNER_TEMP:?}" "${GITHUB_WORKSPACE:?}" "${GITHUB_BASE_SHA:?}"
case "$GITHUB_BASE_SHA" in *[!0-9a-f]*|'') exit 2;; esac
[ "${#GITHUB_BASE_SHA}" = 40 ]
base="$RUNNER_TEMP/duo-baseline-source"
mkdir -p "$base" "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison"
git archive "$GITHUB_BASE_SHA" | tar -x -C "$base"
cmp "$base/mobileapp/package-lock.json" "$GITHUB_WORKSPACE/mobileapp/package-lock.json"
# CocoaPods may rewrite lock metadata during the candidate setup. Compare the
# committed inputs, then use the same resolved lockfile for both builds.
git -C "$GITHUB_WORKSPACE" show HEAD:mobileapp/ios/Podfile.lock > "$RUNNER_TEMP/duo-candidate-committed-pods.lock"
cmp "$base/mobileapp/ios/Podfile.lock" "$RUNNER_TEMP/duo-candidate-committed-pods.lock"
command cp -f "$GITHUB_WORKSPACE/mobileapp/ios/Podfile.lock" "$base/mobileapp/ios/Podfile.lock"
cmp "$base/mobileapp/Gemfile.lock" "$GITHUB_WORKSPACE/mobileapp/Gemfile.lock"
git -C "$GITHUB_WORKSPACE" show HEAD:mobileapp/.bundle/config > "$RUNNER_TEMP/duo-candidate-committed-bundle-config"
cmp "$base/mobileapp/.bundle/config" "$RUNNER_TEMP/duo-candidate-committed-bundle-config"
command cp -f "$GITHUB_WORKSPACE/mobileapp/.bundle/config" "$base/mobileapp/.bundle/config"
python3 "$GITHUB_WORKSPACE/mobileapp/e2e/instrument-baseline.py" "$base"
ln -s "$GITHUB_WORKSPACE/mobileapp/node_modules" "$base/mobileapp/node_modules"
ln -s "$GITHUB_WORKSPACE/mobileapp/vendor" "$base/mobileapp/vendor"
rsync -a "$GITHUB_WORKSPACE/mobileapp/e2e/" "$base/mobileapp/e2e/"
(cd "$base/mobileapp/ios" && bundle exec pod install)
(cd "$base/mobileapp" && xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
  -configuration Release -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -jobs 2 \
  -derivedDataPath "$RUNNER_TEMP/duo-baseline-build" ARCHS=arm64 ONLY_ACTIVE_ARCH=NO CODE_SIGNING_ALLOWED=NO build \
  > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/baseline-build.log" 2>&1)
xcrun simctl list -j devices available > "$RUNNER_TEMP/compare-devices.json"
GTMUX_E2E_UDID=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); m=[x["udid"] for r,ds in d["devices"].items() if r.endswith("iOS-27-0") for x in ds if x["name"]=="iPhone 17" and x["isAvailable"]]; assert len(m)==1, m; print(m[0])' "$RUNNER_TEMP/compare-devices.json")
export GTMUX_E2E_UDID GTMUX_E2E_DEVICE='iPhone 17' GTMUX_E2E_OS='27.0'
set +e
for variant in baseline candidate; do
  # The hosted runner is ephemeral; reset only this explicitly selected CI device
  # so app data and simulator Keychain do not leak from the previous variant.
  xcrun simctl shutdown "$GTMUX_E2E_UDID" >/dev/null 2>&1 || true
  xcrun simctl erase "$GTMUX_E2E_UDID" || exit 1
  xcrun simctl bootstatus "$GTMUX_E2E_UDID" -b || exit 1
  if [ "$variant" = baseline ]; then root="$base/mobileapp"; app="$RUNNER_TEMP/duo-baseline-build/Build/Products/Release-iphonesimulator/gtmux.app"; else root="$GITHUB_WORKSPACE/mobileapp"; app="$RUNNER_TEMP/gtmux-ios-ci/Build/Products/Release-iphonesimulator/gtmux.app"; fi
  xcrun simctl install "$GTMUX_E2E_UDID" "$app"
  install_status=$?
  if [ "$install_status" -eq 0 ]; then (cd "$root" && npm run test:e2e -- --runInBand server-switch-compare.test.ts); test_status=$?; else test_status=$install_status; fi
  printf '%s\n' "$test_status" > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant-status.txt"
  mkdir -p "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant"
  if [ -d "$root/.e2e-artifacts/latest" ]; then command cp -R "$root/.e2e-artifacts/latest/." "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant/"; fi
  if [ "$variant" = candidate ]; then candidate_status=$test_status; fi
done
xcrun simctl shutdown "$GTMUX_E2E_UDID"
exit "$candidate_status"
