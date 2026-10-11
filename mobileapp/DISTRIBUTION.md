# iOS test installation

Maintainer runbook · [中文](./DISTRIBUTION.zh.md)

When an authorized release includes a phone test package, try direct wireless
installation first. If the paired phone cannot be reached, upload the signed IPA to
the existing [ccy.dev distribution service](https://github.com/chenchaoyi/ccy.dev/tree/main/distribution)
and give the tester a restricted Safari installation link. Once published, installation
does not require the Mac to stay online or the phone to connect through gtmux.

## 1. Wireless first, installation link as fallback

- CLI/Mac-only changes do not automatically require a new mobile package. Inspect
  the mobile changes and API compatibility; follow the user's requested scope.
- Check for a reachable, previously paired phone on the network. If available,
  install wirelessly and check its installed version. Otherwise publish an Ad Hoc
  IPA and send the installation link. Do not ask the user to choose each time.
- Reuse the same validated Ad Hoc IPA for either path. An explicit request to test
  the Safari flow goes straight to publication; an explicitly requested development
  build follows [README.md](./README.md) and uses sandbox APNs.
- App Store/TestFlight: follow [submit.md](../docs/appstore/submit.md). An Ad Hoc
  publication does not upload a build to ASC or submit it for review.

Follow branch → PR → green CI → merge → annotated release tag, including user notes,
version stamps and website documentation sync. Check both CLI and notarized Mac
release jobs before `gtmux update`. Report their installed versions separately from
the mobile package. Do not start another release for a documentation-only update.

## 2. Reuse the setup; check space before building

First check available disk space. Reuse dependencies and a matching validated IPA
where possible; uploading an existing IPA needs no native rebuild. Keep archive,
export and DerivedData paths specific to this task. Remove only this task's confirmed
disposable build intermediates; preserve the IPA and publication receipt for retries.
Do not clear another agent's worktree, dependency cache or signing setup.

The deployment is already configured at `https://builds.ccy.dev`, with a dedicated
private test bucket. Do not create a bucket, database or token for every release.
The podcast bucket and website comments database are unrelated resources.

On this release machine the private publisher configuration is
`~/.config/ccy-builds/gtmux-publisher.json` (owner-only, mode `0600`). It contains the
App-scoped publication key and test-bucket S3 credentials. Load them into the uploader's
environment locally; never print them, include them in command arguments, PRs,
receipts, screenshots or public CI artifacts. Do not request the keys again when this
configuration is usable. Prefer a secure credential channel over chat; the phone
App's composer retains drafts/input history and is not a password-entry form.

If the S3 credential is missing, create an **Account API token** in Cloudflare's R2
token manager with **Object Read & Write**, scoped only to the dedicated test bucket.
Wrangler OAuth is not that S3 credential. Its REST upload did not preserve the required
SHA metadata in the first delivery; never weaken the publication validator to accept
that object. See the service runbook for token expiry and replacement.

## 3. Archive and export

Build from the intended release commit. Stamp the version and bilingual release notes
with `scripts/set-version.sh`, then run the required gates. Use the repository's Ruby
and CocoaPods setup; keep Apple's `/usr/bin/rsync` ahead of Homebrew's during export
(the [ASC flow](../docs/appstore/submit.md) documents signing/PATH pitfalls).

Use a generic iOS destination. The archive does not require a connected or unlocked
phone. Set production APNs for distribution; do not reuse the development build's
`APS_ENVIRONMENT=development` setting.

```sh
# From mobileapp; values and paths belong to the current authorized release.
xcodebuild -workspace ios/GtmuxMobile.xcworkspace -scheme GtmuxMobile \
  -configuration Release -destination 'generic/platform=iOS' \
  -derivedDataPath "$RELEASE_DERIVED_DATA" -archivePath "$RELEASE_ARCHIVE" \
  -jobs 2 DEVELOPMENT_TEAM="$APPLE_TEAM_ID" CODE_SIGN_STYLE=Automatic \
  MARKETING_VERSION="$RELEASE_VERSION" CURRENT_PROJECT_VERSION="$RELEASE_BUILD" \
  APS_ENVIRONMENT=production -allowProvisioningUpdates archive

xcodebuild -exportArchive -archivePath "$RELEASE_ARCHIVE" \
  -exportPath "$RELEASE_EXPORT" -exportOptionsPlist "$RELEASE_EXPORT_OPTIONS" \
  -allowProvisioningUpdates
```

Provide the existing ASC authentication flags to **both** commands when headless
automatic signing needs them. The local export-options plist uses
`method = release-testing` (Ad Hoc on this Xcode), `signingStyle = automatic`, the
existing team ID, `destination = export`, and
`manageAppVersionAndBuildNumber = false`. Preserve the stamped build number; do not
export an App Store IPA for direct installation.

Before publishing, verify the IPA's main App **and both extensions**: bundle IDs,
marketing version/build, strict signatures, unexpired distribution profiles,
registered test devices, `get-task-allow = false`, and main App
`aps-environment = production`. The uploader checks the main profile, not every
extension; it does not replace these checks. Record IPA bytes and SHA-256. Do not put
device UDIDs or complete profiles in the repository or public receipts.

## 4. Install wirelessly when reachable

Use bounded device discovery and connection checks. `idevice_id -n` lists network
devices; `idevice_id -l` lists USB devices. A remembered Xcode/CoreDevice record alone
does not prove the phone is reachable. Match the intended, already paired device;
do not pass multiple UDIDs or install on every discovered device.

```sh
idevice_id -n
# After matching the intended device and confirming that it responds:
ideviceinstaller -n -u "${TEST_DEVICE_UDID:?select the paired network test device}" \
  install "$RELEASE_IPA"
ideviceinstaller -n -u "$TEST_DEVICE_UDID" list -b com.gtmux.app \
  -a CFBundleShortVersionString -a CFBundleVersion --xml
```

Bound the commands with the runner's timeouts, not an unsupported installer flag.
If discovery or connection fails, continue to publication below without waiting for
USB, repeatedly polling, or asking the user to bring the phone to the Mac. If the
installer times out after starting, check its result and the installed version once:
an unknown outcome is not a failed installation. Report uncertainty if that check
also fails, and supply a link to the same IPA as recovery. Device discovery or command
submission alone is not installation success. Unlocking may still be needed for
visual/device-feature acceptance; keep that separate from installation.

## 5. Publish through ccy.dev when direct installation is unavailable

Routine releases use the already deployed service. Run its existing uploader;
no ccy.dev source change, PR, website deployment or Worker deployment is needed
for each package. Change that repository only when changing the distribution service.

Use the service's documented uploader from the **ccy.dev checkout**, not a gtmux
command or tunnel upload. See its
[publisher workflow](https://github.com/chenchaoyi/ccy.dev/blob/main/distribution/README.md#发布者使用--publisher-workflow)
for the environment variable names and private config example.

```sh
# Inspect first; no credentials or remote writes.
python3 distribution/scripts/builds.py --app gtmux publish "$RELEASE_IPA" \
  --receipt "$RELEASE_RECEIPT" --dry-run

# With the private publisher environment loaded; capture stdout in a mode-0600
# local result file because it includes the bearer installation URL.
umask 077
set -C  # Use a new private result path; refuse to overwrite an existing file.
python3 distribution/scripts/builds.py --app gtmux publish "$RELEASE_IPA" \
  --name gtmux --notes-zh "$RELEASE_NOTES_ZH" --notes-en "$RELEASE_NOTES_EN" \
  --receipt "$RELEASE_RECEIPT" > "$RELEASE_PUBLICATION_RESULT"
```

The uploader saves the receipt before uploading, verifies R2 size/SHA metadata and
publishes the version before issuing a **single-build, 24-hour** installation link.
Keep the result private. The link is a download credential, not a public release asset;
give it only to the intended tester, not GitHub, website navigation or an HQ report.

Retry the **same IPA and receipt** after an interrupted upload/publication. If the
version is already published but its link failed or expired, use `link --build <id>`;
do not rebuild or upload another copy just to issue a link. Do not overwrite a receipt's
object when its bytes/SHA differ. Use a new receipt for a distinct artifact or changed
metadata. `--all-builds` is broader access, including future versions, and needs an
explicit request.

## 6. Check delivery and hand off

Check the live Chinese/English page, same-origin HTTPS manifest, correct bundle/build,
IPA HEAD/size, complete download SHA and Range/If-Range. Confirm unauthenticated
publisher requests are refused and a pinned link cannot select another version.
After changing authorization/download code, test revocation on a separate disposable
grant: page, manifest and IPA must all refuse new requests. Preserve the tester's link.
Do not rerun the entire protocol suite for each unchanged package.

Send the tester the link, exact version/build and expiry with timezone. Ask them to
open it in **Safari on a registered iPhone/iPad**, tap **Install test build**, accept
the system prompt and open the App. Do not ask them to delete the old App first.
Do not fall back to USB installation when the task is to test the remote flow.

Report these independently:

| Evidence | What it proves |
| --- | --- |
| CI + exported signatures/profiles | Build and signing checks |
| Live page/manifest/download checks | Publication and download protocol |
| User confirms installation | Installation on that device; say if the version was not separately checked |
| In-App version and retained pairing/data | Correct update and data preservation |
| Real notification and Live Activity | Those device features with this signed package |
| iPad layout / VoiceOver | Separate platform/accessibility acceptance |

v1.0.104 (build 104) completed S3 publication and production protocol checks on
2026-10-11; the user confirmed successful iPhone installation. This does **not** mark
retained data, APNs, Live Activity, iPad, VoiceOver or full zone/CDN/Logpush logging
review as passed. Future receipts must retain these boundaries and report remaining
disk space. Full zone logging review remains an operator follow-up; Worker
observability is disabled and application code does not log installation URLs.
