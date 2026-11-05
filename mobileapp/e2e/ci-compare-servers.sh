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
# Metro only indexes files inside its project and watch folders. The isolated
# baseline links the candidate's identical dependencies, so make that real
# dependency directory visible to Metro without copying or reinstalling it.
cat >> "$base/mobileapp/metro.config.js" <<'EOF'
module.exports.watchFolders = [...(module.exports.watchFolders || []), process.env.GTMUX_COMPARE_NODE_MODULES];
module.exports.resolver.nodeModulesPaths = [...(module.exports.resolver.nodeModulesPaths || []), process.env.GTMUX_COMPARE_NODE_MODULES];
EOF
export GTMUX_COMPARE_NODE_MODULES="$GITHUB_WORKSPACE/mobileapp/node_modules"
rsync -a "$GITHUB_WORKSPACE/mobileapp/e2e/" "$base/mobileapp/e2e/"
(cd "$base/mobileapp/ios" && bundle exec pod install)
# Test both products with the same Xcode simulator signing mode. This uses only
# the existing target entitlements and no identity or provisioning credentials.
signing=(CODE_SIGNING_ALLOWED=NO)
candidate_app="$RUNNER_TEMP/gtmux-ios-ci/Build/Products/Release-iphonesimulator/gtmux.app"
if [ "${GTMUX_COMPARE_ADHOC:-0}" = 1 ]; then
  signing=(CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=-)
  candidate_app="$RUNNER_TEMP/duo-candidate-adhoc/Build/Products/Release-iphonesimulator/gtmux.app"
  (cd "$GITHUB_WORKSPACE/mobileapp" && xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
    -configuration Release -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -jobs 2 \
    -derivedDataPath "$RUNNER_TEMP/duo-candidate-adhoc" ARCHS=arm64 ONLY_ACTIVE_ARCH=NO "${signing[@]}" build \
    > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/candidate-build.log" 2>&1)
fi
(cd "$base/mobileapp" && xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
  -configuration Release -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -jobs 2 \
  -derivedDataPath "$RUNNER_TEMP/duo-baseline-build" ARCHS=arm64 ONLY_ACTIVE_ARCH=NO "${signing[@]}" build \
  > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/baseline-build.log" 2>&1)
xcrun simctl list -j devices available > "$RUNNER_TEMP/compare-devices.json"
compare_os=${GTMUX_COMPARE_OS:-27.0}
compare_device=${GTMUX_COMPARE_DEVICE:-iPhone 17}
GTMUX_E2E_UDID=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); os,device=sys.argv[2:]; suffix="iOS-"+os.replace(".","-"); m=[x["udid"] for r,ds in d["devices"].items() if r.endswith(suffix) for x in ds if x["name"]==device and x["isAvailable"]]; assert len(m)==1, m; print(m[0])' "$RUNNER_TEMP/compare-devices.json" "$compare_os" "$compare_device")
export GTMUX_E2E_UDID GTMUX_E2E_DEVICE="$compare_device" GTMUX_E2E_OS="$compare_os" GTMUX_COMPARE_MANUAL_LAUNCH='1'
set +e
overall_status=0
for variant in baseline candidate; do
  # The hosted runner is ephemeral; reset only this explicitly selected CI device
  # so app data and simulator Keychain do not leak from the previous variant.
  xcrun simctl shutdown "$GTMUX_E2E_UDID" >/dev/null 2>&1 || true
  xcrun simctl erase "$GTMUX_E2E_UDID" || exit 1
  xcrun simctl bootstatus "$GTMUX_E2E_UDID" -b || exit 1
  if [ "$variant" = baseline ]; then root="$base/mobileapp"; app="$RUNNER_TEMP/duo-baseline-build/Build/Products/Release-iphonesimulator/gtmux.app"; else root="$GITHUB_WORKSPACE/mobileapp"; app="$candidate_app"; fi
  # Inspect the built product, not the source entitlements file. This is read-only
  # and records absent signatures as well as any actual simulator entitlements.
  codesign -d --entitlements :- "$app" > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant-entitlements.txt" 2>&1 || true
  codesign -dv "$app" > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant-signature.txt" 2>&1 || true
  xcrun simctl install "$GTMUX_E2E_UDID" "$app"
  install_status=$?
  if [ "$install_status" -eq 0 ]; then (cd "$root" && npm run test:e2e -- --runInBand server-switch-compare.test.ts); test_status=$?; else test_status=$install_status; fi
  printf '%s\n' "$test_status" > "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant-status.txt"
  if [ "$test_status" -ne 0 ]; then
    # A failed WDA session is not proof of an app crash. Capture a separate,
    # time-bounded Simulator launch and error log before this CI device is erased.
    python3 - "$GTMUX_E2E_UDID" "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant" <<'PY'
import glob, os, re, subprocess, sys, time
udid, prefix = sys.argv[1:]
commands = {
    'launch': ['xcrun', 'simctl', 'launch', udid, 'com.gtmux.app'],
    'errors': ['xcrun', 'simctl', 'spawn', udid, 'log', 'show', '--last', '2m', '--style', 'compact', '--predicate', 'process == "gtmux" OR ((process == "SpringBoard" OR process == "runningboardd") AND eventMessage CONTAINS[c] "com.gtmux.app")'],
}
for name, command in commands.items():
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=30)
        output = 'exit=%s\n%s\n%s' % (result.returncode, result.stdout[-50000:], result.stderr[-5000:])
    except subprocess.TimeoutExpired:
        output = 'timed out after 30 seconds\n'
    with open(prefix + '-' + name + '.txt', 'w') as file:
        file.write(output)
    if name == 'launch':
        match = re.search(r'com\.gtmux\.app:\s*(\d+)', output)
        if match:
            time.sleep(5)
            status = subprocess.run(['ps', '-p', match.group(1), '-o', 'pid=,stat=,command='], capture_output=True, text=True, timeout=10)
            with open(prefix + '-process.txt', 'w') as file:
                file.write('exit=%s\n%s\n%s' % (status.returncode, status.stdout, status.stderr))
# Runner is ephemeral. Include only recent app-specific crash reports, bounded in size.
reports = sorted(glob.glob(os.path.expanduser('~/Library/Logs/DiagnosticReports/gtmux*.ips')), key=os.path.getmtime, reverse=True)
for index, report in enumerate(reports[:3]):
    if time.time() - os.path.getmtime(report) < 900:
        with open(report, 'rb') as source, open(prefix + '-crash-%s.ips' % index, 'wb') as dest:
            dest.write(source.read(200_000))
PY
  fi
  mkdir -p "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant"
  if [ -d "$root/.e2e-artifacts/latest" ]; then command cp -R "$root/.e2e-artifacts/latest/." "$GITHUB_WORKSPACE/mobileapp/.e2e-artifacts/comparison/$variant/"; fi
  if [ "$test_status" -ne 0 ]; then overall_status=1; fi
done
xcrun simctl shutdown "$GTMUX_E2E_UDID"
exit "$overall_status"
