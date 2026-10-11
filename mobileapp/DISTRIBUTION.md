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

## Hosted delivery: the preferred cross-machine path after setup

The optional `iOS test delivery (maintainers)` workflow builds and publishes the
same registered-device Ad Hoc package on an ephemeral GitHub macOS runner. Once
configured, an authorized maintainer can trigger it from another computer or the
GitHub website without copying this Mac's publisher file, Xcode or signing keychain.
It is **disabled until setup** (`IOS_TEST_DELIVERY_ENABLED=true`). Merging the
workflow does not migrate credentials, provision devices, publish a package or
submit an App Store build. Until the hosted path has passed a real run, use the
local fallback below.

### One-time operator setup (requires review)

Create the `ios-test-delivery` environment. Restrict deployments to `main`, require
maintainer approval, prevent self-review where appropriate, and review any admin
bypass. Ordinary public/fork PR checks need none of these credentials. Do not put
them in repository variables or committed files. The workflow is manually dispatched
from `main` and independently rejects unpublished tags, commits outside main and
release commits whose latest main CI has not passed. UI previews need no release
tag: they accept only an exact main commit whose latest main CI passed. Signing
unreviewed feature branches or fork commits is outside this workflow.

| Environment setting | Source / scope |
| --- | --- |
| Secret `IOS_CERT_P12`, `IOS_CERT_PASSWORD` | Base64 iOS **Apple Distribution** certificate/private key and its password; not the Mac Developer ID identity |
| Secrets `IOS_PROFILE_APP`, `IOS_PROFILE_WIDGET`, `IOS_PROFILE_NOTIFICATION` | Base64 Ad Hoc profiles for `com.gtmux.app`, `.widget` and `.notificationservice`; matching team, unexpired, common registered devices, production APNs on the main App |
| Secret `IOS_PUBLISHER_READ_TOKEN` | Fine-grained token with **Contents: read** only on the private ccy.dev repository; the default token cannot read it |
| The five `BUILDS_*` secrets below | Existing dedicated test-bucket publisher settings, with the gtmux-scoped API key; reuse after approval, not a new bucket per build |
| Variable `IOS_TEAM_ID` | Signing team's ten-character identifier |
| Variable `IOS_RESULT_RECIPIENT` | Maintainer's **public** native age recipient; private identity stays outside GitHub |

Set repository variable `IOS_TEST_DELIVERY_ENABLED=true` **last**, after reviewing
the environment and uploading secrets through GitHub's secret settings or a secure
stdin-based operator command. Never paste them into chat or a shell argument. No
credential migration was performed while adding this workflow. Fork owners who
want hosted delivery must supply their own signing, private-uploader access and
publication service setup; normal build/test contributions remain independent of it.

Generate an age identity once using the [official age tool](https://github.com/FiloSottile/age):

```sh
umask 077
age-keygen -o "$PRIVATE_DELIVERY_IDENTITY"
# It prints the PUBLIC recipient; set IOS_RESULT_RECIPIENT to that public value.
```

Keep the private identity in the operator's approved secret store, with a recovery
copy. It only decrypts workflow results; it is not a publisher or signing credential.
Another machine needs this identity only when retrieving a result, not when
triggering the workflow. Losing it means an existing encrypted result cannot be
opened. Never upload it to GitHub. Rotate the public recipient when appropriate.

### Trigger and retrieve

In GitHub: **Actions → iOS test delivery (maintainers) → Run workflow → main**,
choose a purpose and approve the environment deployment:

- `ui-preview` (default): leave `preview_commit` blank for current main, or supply a
  full main commit SHA. It must have green CI. Leave `release_tag` blank. No tag,
  GitHub Release, store notes stamp or ASC submission is created. The package uses
  the source App version, a distinct `run_number.run_attempt` build number, and a UI-preview label
  with the commit ID in its installation page/notes.
- `release-test`: supply a published `vX.Y.Z` in `release_tag`; the App version must
  already match. It uses the committed bilingual release notes.

Or:

```sh
# UI validation without a release:
gh workflow run ios-test-delivery.yml --ref main -f purpose=ui-preview
# An already published release:
gh workflow run ios-test-delivery.yml --ref main -f purpose=release-test -f release_tag=vX.Y.Z
gh run list --workflow ios-test-delivery.yml
# Select the actual run ID; do not accidentally download another concurrent release.
gh run download "$DELIVERY_RUN_ID" -n "$DELIVERY_ARTIFACT_NAME" -D "$PRIVATE_RESULT_DIR"
age --decrypt -i "$PRIVATE_DELIVERY_IDENTITY" \
  -o "$PRIVATE_RESULT_DIR/result.tar.gz" "$PRIVATE_RESULT_DIR/ios-delivery.tar.gz.age"
tar -xzf "$PRIVATE_RESULT_DIR/result.tar.gz" -C "$PRIVATE_RESULT_DIR"
```

Create a new owner-only result directory first and keep `umask 077`. The artifact
name includes the run ID and attempt, and expires after three days. **Only encrypted
files are public artifacts.** The decrypted `publication.json` contains the single-build
24-hour Safari link; share it privately with the tester. The decrypted export includes
the IPA, so a reachable paired phone can still be installed wirelessly by a trusted
Mac. GitHub's runner has neither this Mac's pairing nor direct access to the phone.

Public logs show stage/status, never the installer link, raw IPA, profiles or build
logs. Failure results preserve encrypted logs, an exported IPA if available and the
publication receipt if created. Credentials and profile setup files are excluded.
If publication succeeded but link issuance failed, the job **fails** and the receipt
allows an authorized operator to issue a link without rebuilding. An interrupted
workflow may have uploaded the object: recover the same receipt/IPA rather than
blindly publishing again. Cancellation before encryption may leave no recovery
artifact; report that limitation rather than claiming a successful publication.

The workflow uses manual profile signing, a pinned private uploader and immutable
Action revisions. It creates no Apple devices, ASC submission or deployment of
ccy.dev. Exact runner Xcode/Ruby availability and real signing/export need first-run
verification; synthetic tests cannot prove them. Hosted upload success is separate
from Safari installation, retained pairing, APNs/Live Activity, iPad layout and VoiceOver.

## 2. Local fallback: reuse the setup; check space before building

First check available disk space. Reuse dependencies and a matching validated IPA
where possible; uploading an existing IPA needs no native rebuild. Keep archive,
export and DerivedData paths specific to this task. Remove only this task's confirmed
disposable build intermediates; preserve the IPA and publication receipt for retries.
Do not clear another agent's worktree, dependency cache or signing setup.

The deployment is already configured at `https://builds.ccy.dev`, with a dedicated
private test bucket. Do not create a bucket, database or token for every release.
The podcast bucket and website comments database are unrelated resources.

Publisher credentials are **machine-local**, not supplied by a Git clone or by HQ.
The default is `$XDG_CONFIG_HOME/ccy-builds/gtmux-publisher.json`, falling back to
`~/.config/ccy-builds/gtmux-publisher.json`; this is a convention, not evidence that
another Mac has the file. `--config` or `BUILDS_PUBLISHER_CONFIG` can select another
private location. The adapter loads only the five documented publisher fields;
complete local/CI environment settings work without a file and take precedence.
`BUILDS_ORIGIN` can override the HTTPS origin. Never print secrets or put them in
command arguments, Git, receipts, screenshots or public CI artifacts. The phone App's
composer retains drafts/input history and is not a password-entry form.

### New release machine: preflight and one-time setup

Before an expensive build, identify the intended installation path. If direct
installation is reachable, publication credentials are not required. If the Safari
fallback or an explicit Safari test is needed, preflight the publisher **before**
archiving. Also check Xcode, the repository Ruby version, Apple signing access and
registered test devices separately: publisher settings do not provision Apple signing.

From the gtmux checkout, check local settings without displaying values or making a
network request:

```sh
python3 mobileapp/scripts/test-publisher.py check
# Optional explicit location:
python3 mobileapp/scripts/test-publisher.py --config "$PRIVATE_PUBLISHER_CONFIG" check
```

If settings are absent, reuse an existing authorized configuration through an
**already verified** SSH connection or an approved password-manager/secret channel.
Only trusted release operators get this file: its S3 credentials allow access to the
entire test bucket, not just gtmux. Testers need only their restricted installation
link. This runbook does not establish remote login, create credentials or authorize
sharing them with a new person.

On the new Mac, the following imports the existing JSON directly from SSH stdout;
no value enters a command argument or clipboard. Set `PUBLISHER_HOST` to the real,
verified source host/alias. No working source-Mac SSH host is currently recorded;
this example is **not** a claim that either Mac is reachable.

```sh
# Run from the NEW Mac's gtmux checkout; requires an existing verified SSH channel.
set -o pipefail
ssh "${PUBLISHER_HOST:?set the verified source release Mac}" \
  'cat "$HOME/.config/ccy-builds/gtmux-publisher.json"' | \
  python3 mobileapp/scripts/test-publisher.py import-config
python3 mobileapp/scripts/test-publisher.py check
```

The source path is the default example: if its config lives elsewhere, use its
verified private path. Import validates the JSON, creates an owner-only (`0600`)
file, and refuses an existing destination, incomplete/empty input and symlinks. An
approved secret manager can instead supply JSON to the same stdin command or expose
`BUILDS_*` environment variables. Do not improvise a transfer using VPS credentials,
a public download or chat. If no channel exists, report that specific missing
prerequisite; preserve a validated IPA for later publication. A missing local file
alone is not a reason to request a new R2 key or deploy ccy.dev again.

Get a current ccy.dev checkout at a location selected on **this** Mac. Routine
uploads use its existing uploader; they do not need a source change or redeployment.
For example, set `CCY_DEV_CHECKOUT` to that checkout, then prepare its Python runtime:

```sh
# Use this Mac's paths; never copy another machine's absolute checkout/venv paths.
BUILDS_VENV="${XDG_DATA_HOME:-$HOME/.local/share}/ccy-builds/uploader-venv"
python3 -m venv "$BUILDS_VENV"
"$BUILDS_VENV/bin/python" -m pip install \
  -r "${CCY_DEV_CHECKOUT:?set the local ccy.dev checkout}/distribution/scripts/requirements.txt"
BUILDS_UPLOADER="$CCY_DEV_CHECKOUT/distribution/scripts/builds.py"
BUILDS_PYTHON="$BUILDS_VENV/bin/python"
"$BUILDS_PYTHON" mobileapp/scripts/test-publisher.py check --uploader "$BUILDS_UPLOADER"
```

This check verifies local configuration, the uploader path and boto3 availability.
It does **not** prove remote token validity, Apple signing or installation. Keep
those results separate. Reuse this setup on later releases; do not recreate it each time.

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

The uploader remains in the **ccy.dev checkout**. The gtmux maintainer adapter
only loads configuration and saves private output; it does not implement another
upload protocol or add a gtmux App/CLI feature. See the service's
[publisher workflow](https://github.com/chenchaoyi/ccy.dev/blob/main/distribution/README.md#发布者使用--publisher-workflow)
for the five `BUILDS_*` fields. Use the `BUILDS_PYTHON` and `BUILDS_UPLOADER` paths
from this machine's preflight above. A custom JSON path is accepted before the adapter
subcommand with `--config`, or through `BUILDS_PUBLISHER_CONFIG`.

```sh
# Inspect first with the original uploader; no credentials or remote writes.
"$BUILDS_PYTHON" "$BUILDS_UPLOADER" --app gtmux publish "$RELEASE_IPA" \
  --receipt "$RELEASE_RECEIPT" --dry-run

# From the gtmux checkout. Both output paths must be new and their parent must exist.
# The adapter creates each file with mode 0600, regardless of shell umask.
"$BUILDS_PYTHON" mobileapp/scripts/test-publisher.py run \
  --uploader "$BUILDS_UPLOADER" --output "$RELEASE_PUBLICATION_RESULT" \
  --log "$RELEASE_PUBLICATION_LOG" -- \
  publish "$RELEASE_IPA" --name gtmux \
  --notes-zh "$RELEASE_NOTES_ZH" --notes-en "$RELEASE_NOTES_EN" \
  --receipt "$RELEASE_RECEIPT"
```

Neither stdout nor stderr from the uploader is echoed into the agent/CI transcript.
Read the private result locally to hand the URL only to the intended tester. The
adapter returns the uploader's exit status; on failure it retains private output and
the existing IPA/receipt rather than pretending publication succeeded. Use fresh
result/log paths for a retry; keep the same IPA/receipt. After a published version's
link expires, the same adapter can run `link --build <id> --hours 24`.

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
