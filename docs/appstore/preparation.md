# App Store preparation and recovery

Run commands from `mobileapp/`. Final preview and Submit belong to the operator.
This flow never calls the submission script. [中文](preparation.zh.md)

## 1. Check inputs before expensive work

Choose the published version and exact source. Review the user-visible changes since
the **live Store version**, not only the last release. Review both locales' text and
`shot-captions.json` / `creative-captions.json`, including which screens they describe.
Use the existing `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY_PATH` in the current process;
verify the private key exists. Do not depend on another machine's shell profile or
an earlier task's temporary directory. Missing settings fail before API access.
Do not put the key, profiles or authenticated download URLs into Git or public artifacts.

## 2. Build and validate the binary before upload

The canonical path is `bundle exec fastlane release`. It retains
`build/GtmuxMobile.xcarchive`, exports the IPA and runs `scripts/store_package.py`
**before** uploading. The app and both extensions must have exact version/build,
valid Store profiles, valid signatures and matching archive symbols. Executable
content is compared on private copies with signatures removed; resources and Info
fields must match, except the chosen build number. Signed DER rights must match
the baseline with only `beta-reports-active=true` allowed for Store distribution.
Unexpected keychain/app groups, debugger access or APNs changes block upload.

For the limited-disk fallback, start with the verified original release IPA, follow
[resigning guidance](reuse-ipa.md), then use the gated upload lane:

```sh
bundle exec fastlane upload ipa:/private/path/store.ipa \
  baseline:/private/path/original.ipa version:X.Y.Z build:N
```

The lane creates `build/store-package.json` only after successful validation.
Keep it with the exact IPA; the final status command rechecks its SHA-256 and
candidate identity. An original-IPA fallback cannot recover missing archive symbols;
the status records that limitation. Do not claim retained pairing or device acceptance
from signatures or Apple's VALID processing result.

Keep the canonical archive and dSYMs in encrypted off-machine backup. Their UUIDs
must match the package ([Apple's guidance](https://developer.apple.com/documentation/xcode/adding-identifiable-symbol-names-to-a-crash-report)).
The existing hosted test-delivery recovery now includes archive Products, Info and
dSYMs, excludes embedded profiles/signing setup/symlinks, and is encrypted to the
existing operator-held recipient. Download/decrypt and verify before its **14-day**
artifact expiry. The sanitized archive needs separately authorized profiles to export
again. Do not treat this change as a newly configured hosted Store upload workflow.

## 3. Reuse cloud capture work

See [Actions screenshots](assets-ci.md). Builds, completed device/locale groups and
rendered assets are separate artifacts. A failed later stage does not require
rebuilding the app. Rendering uses current reviewed scripts/captions and retains the
released app/widget and original capture identities in the manifest.

Images still require visual inspection at original size. The capture verifies known
demo content is present; it cannot establish every layout, VoiceOver or real push result.

## 4. Place assets and independently read everything back

Use existing metadata and build attachment commands from [the submission runbook](submit.md).
Then run:

```sh
mkdir -p build
bundle exec ruby scripts/asc-preparation.rb assets \
  --version X.Y.Z --build N --ipa /private/path/store.ipa \
  --package-receipt build/store-package.json \
  --state build/store-state.json --journal build/store-progress.jsonl
```

`assets` places screenshots and Header/Search Results images using the existing
content-based reuse logic, then independently checks the selected VALID build,
version-localized text, four screenshot groups and four creative placements.
It also compares the package receipt to the actual IPA. Missing images, weak
legacy name/size matches or inactive placements prevent readiness.
`status` performs the same independent readback without placement writes:

```sh
bundle exec ruby scripts/asc-preparation.rb status \
  --version X.Y.Z --build N --ipa /private/path/store.ipa \
  --package-receipt build/store-package.json --state build/store-state.json
```

The private state is replaced on every check; network errors cannot leave an earlier
ready result current. It reports each check and only emits `ready_for_user_review`
when all pass. The scope is package/build, version text, screenshots and creative;
it does not validate account agreements, age-rating/privacy forms, review contact,
app-info name/subtitle or ASC web preview. Follow the runbook for those checks.
Never interpret a saved receipt as a live readiness result without running `status`.

## Processing and recovery

Progress is flushed immediately, with image IDs and processing states; the optional
journal is owner-only mode 0600. It contains no API tokens or signed upload URLs.
Long waits refresh the request JWT in memory with the same key before expiry; no new credential or scope is created.
The default processing wait is 900 seconds per image, configurable with
`ASC_PROCESSING_WAIT_SECONDS=1..1800`. An expired processing wait exits **75 (pending)**.
Rerun the same command and inputs: it waits for the existing image and reuses completed
uploads. Existing placements remain until replacement images for their group are ready.
A reservation without committed bytes can be recreated; a processing image is not
reuploaded merely because the wait expired. Unknown/failed processing is not ready.
No automatic submission is part of this recovery.
