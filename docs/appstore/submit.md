# App Store submission (iPhone + iPad)

[中文](submit.zh.md)

Maintainer runbook. Commands below run from the repository root; subshell `cd` does
not change the outer working directory. Follow [preparation and recovery](preparation.md)
for reusable builds, package validation, asset recovery and the final readiness gate.
**An agent preparing a submission stops at operator review; the user owns final Submit.**

## 0. Prerequisites

- Select the published release and exact source. `bash mobileapp/scripts/set-version.sh`
  uses the nearest reachable tag, stamps the project and archives changed bilingual
  release notes. Commit through a PR and run `scripts/check-design.sh` after stamping;
  it checks the app-source fingerprint against the archived notes.
- Configure the existing Team key in the current fastlane/ASC process:
  `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY_PATH`. Do not assume another machine's shell
  profile or a previous task's temporary directory. Never commit the private key.
- Use the Ruby version in `mobileapp/.ruby-version` and `(cd mobileapp && bundle install)`;
  the system Ruby is too old for fastlane. Noninteractive shells need an explicit PATH.
- All three targets are universal (`TARGETED_DEVICE_FAMILY = "1,2"`); prepare the
  13-inch iPad screenshot slot as well as iPhone. Follow
  [Apple's screenshot specifications](https://developer.apple.com/help/app-store-connect/reference/app-information/screenshot-specifications/).

## 1. Listing text

Review `mobileapp/fastlane/metadata/{en-US,zh-Hans}/`:

| File | Purpose |
|---|---|
| `description.txt` | Write both locales naturally; include iPad support. |
| `release_notes.txt` | Summarize user-visible changes since the **live Store version**. |
| `keywords.txt`, `subtitle.txt`, `promotional_text.txt` | Change only when appropriate. |

For a submission spanning several releases, archive the combined Store notes under
`mobileapp/release-notes/store/<version>.*.txt`, then restore this release's individual
notes in fastlane metadata. See `mobileapp/release-notes/README.md`.

## 2. Screenshots

Use the self-contained demo, with no real sessions or credentials:

| Slot | Size | Files in each locale |
|---|---|---|
| iPhone 6.9-inch | 1320×2868 | `01..07.png` |
| iPad 13-inch landscape | 2752×2064 | `ipad-01..04.png` |

Prefer [reusable Actions captures](assets-ci.md). Local capture instructions are in
[App Store screenshots](../appstore-shots.md). For iPad, use an explicitly selected,
booted disposable simulator, never the output of an unfiltered device list:

```sh
(
  set -eu
  cd mobileapp
  : "${AUDIT_IPAD_UDID:?Select this task's booted iPad simulator}"
  GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" npm run e2e:build
  for L in en zh; do
    GTMUX_DEMO_SHOTS=1 GTMUX_SHOTS_LANG="$L" GTMUX_E2E_UDID="$AUDIT_IPAD_UDID" \
      GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)' npm run test:e2e -- appstore-shots-ipad.test.ts
  done
  node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-en --lang ipad-en --out fastlane/screenshots/en-US --prefix ipad-
  node scripts/frame-shots.mjs --slot ipad --in .e2e-artifacts/appstore/ipad-zh --lang ipad-zh --out fastlane/screenshots/zh-Hans --prefix ipad-
)
```

Review `scripts/shot-captions.json`, including `ipad-en` and `ipad-zh`. Inspect every
image at original size and confirm landscape orientation. iOS 26.5 can already return
landscape pixels; blindly applying the older 270-degree compensation rotates them twice.
The script checks actual image dimensions. See [troubleshooting](../TROUBLESHOOTING.md).

## 2b. Header and Search Results assets

Each locale needs one 3840×2160 creative image, placed in both Header and Search Results.
Content is 4+; avoid price, URLs, copyright marks, other platforms' logos or Apple awards.
Headers may crop to 21:9, so keep essential content away from the top/bottom ~350px.

Reuse unchanged bytes. If screens or captions change, render and visually inspect:

```sh
(
  set -eu
  cd mobileapp
  node scripts/frame-creative.mjs --lang en --ipad .e2e-artifacts/appstore/ipad-en \
    --phone .e2e-artifacts/appstore/en --out fastlane/creative/en-US
  node scripts/frame-creative.mjs --lang zh --ipad .e2e-artifacts/appstore/ipad-zh \
    --phone .e2e-artifacts/appstore/zh --out fastlane/creative/zh-Hans
)
```

Place before submission, while the version is editable. Library image existence alone
is insufficient: both placements per locale must read back `ok`. Once submitted/live,
the version's placements cannot be added through that API; separate asset review and
product-page selection in ASC may be needed (the 1.0.97 recovery).

## 3. Build and upload the binary

```sh
(
  set -eu
  cd mobileapp
  : "${ASC_KEY_ID:?Configure existing Team key}" "${ASC_ISSUER_ID:?Configure issuer}" "${ASC_KEY_PATH:?Select key file}"
  bundle exec fastlane release
)
```

The lane builds, archives, exports, validates and uploads without another manual archive.
It selects the next build number from ASC and uses Release's production APNs configuration;
do not override `APS_ENVIRONMENT`. For an existing IPA, use the validated upload path in
[preparation](preparation.md), including its baseline and receipt. Neither lane waits for
processing completion or submits for review. Preserve archive/dSYMs as documented there.

Development builds and installation are in [mobileapp README](../../mobileapp/README.md).
Choose one explicit test-device UDID. The same app installs on iPhone and iPad; a dev
or Ad Hoc package is not a Store package. Historical installation-tool successes do not
prove arbitrary locked/rebooted devices can install.

## 4. Listing text, screenshots and build attachment

```sh
(
  set -eu
  cd mobileapp
  : "${STORE_BUILD:?Select processed build}" "${STORE_VERSION:?Select Store version}"
  bundle exec fastlane metadata
  bundle exec ruby scripts/asc-attach-build.rb "$STORE_BUILD"
  bundle exec ruby scripts/asc-attach-build.rb --list
)
```

`fastlane metadata` creates the version and uploads text. Since API 4.5.1 deprecated
deliver's screenshot endpoints, `asc-asset-library.rb` places content-hash-named assets
without repeated uploads. Use `--dry-run` to inspect placement plans. Every locale/slot
must report `ok`; `TODO` exits 1. Use the gated `asc-preparation.rb assets` in §4b for canonical placement. The individual
placement commands are lower-level recovery tools and do not establish readiness.
The older `fastlane metadata deliver_screenshots:true`
is a fallback only while deprecated endpoints remain available; independently read back
its output too. Assets existing in the library do not prove placement on this version.

## 4b. Accessibility review and unified readiness

After metadata/build attachment and before asset placement, create and complete a
candidate-package-bound private [accessibility review](accessibility.md). Every submission needs one,
with separate iPhone/iPad evidence. Do not copy the previous answers as approval. The
optional Accessibility URL can remain empty. Run `asc-preparation.rb assets/status` as
shown in [preparation](preparation.md#4-place-assets-and-independently-read-everything-back);
only `ready_for_user_review` is ready to hand off. Keep an owner and plan for new claims;
publish them only once the exact version/build is live, run `asc-accessibility.rb live-status`
and separately record the Store preview. These helpers never publish labels or submit.

## 5. Operator preview in ASC

- Exact candidate build selected; no previous-build fallback.
- Both locales: seven phone and four iPad images, correct order, no duplicates; screenshot status all `ok`.
- Header/Search Results: four `ok` placements and both locales visually previewed.
- iPad slot is 13-inch; Requires full screen remains unchecked for multitasking.
- What's New covers the span from the live Store version, not only the latest tag.
- Accessibility review matches this package; live claims remain accurate and new claims have post-live follow-up.
- Privacy-policy URL still opens: [privacy policy](privacy-policy.md).

## 6. Reviewer experience: built-in demo only

No demo server or guest link. On first pairing, tap **View demo**
(Chinese: **查看演示**; verify the current `PairingScreen.tsx` label).
The offline synthetic demo covers radar, terminal/replies, HQ, knowledge, usage and iPad
layout. Review contact and Sign-in required = No may carry over; check them each time.

## 7. Submit for review

A preparation agent stops before this step; the user previews and clicks Submit in ASC.
The following is the operator's separate, deliberate CLI alternative:

```sh
(
  set -eu
  cd mobileapp
  bundle exec ruby scripts/asc-submit-review.rb "${STORE_VERSION:?Select Store version}" "${STORE_BUILD:?Select build}"
)
```

It refuses a mismatched build, adds the version to a review submission and submits with
bounded retries for Apple's intermittent HTTP 500. Once added, the version becomes
READY_FOR_REVIEW and is no longer editable; look it up by version string. Independently
read back WAITING_FOR_REVIEW. Archive combined release notes as described in §1.

## Known pitfalls

- Do not revert a changed Podfile.lock just before archive: it may differ from installed
  Pods/Manifest.lock. Compare them first; synchronize with `bundle exec pod install`.
- Development signatures and sandbox APNs are not Store signatures/production APNs.
- If headless extension signing initially fails, configure all three targets in Xcode;
  never silently remove an extension. See [troubleshooting](../TROUBLESHOOTING.md).
- A new Xcode can require platform downloads/Pod deployment-target adjustments.
- Upload success is not build attachment; always read back. Older deliver retries
  produced duplicates; check actual placements instead of assuming success.

## One-time fields in ASC

| Field | Existing setting / review rule |
|---|---|
| Category | Developer Tools, no secondary category needed |
| Age Rating | All None → 4+ |
| App Privacy | Data Not Collected (operator decision, 2026-10-07); do not change as an accessibility side effect |
| Export Compliance | Standard HTTPS/TLS exemption; `ITSAppUsesNonExemptEncryption=false` |
| Pricing | Free |
| Availability | All countries/regions except mainland China; preserve existing access policy |
| Sign-in required | No account; pairing connects to the user's own server |

The existing privacy answer reflects the app's actual data flow: the phone communicates
with its user's server; the relay forwards APNs traffic without retaining it; Worker
observability/Logpush is off; tunnel identity is managed by the server CLI, not the app;
images go only to the user's server; tokens remain in Keychain; no analytics/tracking SDK.
If retention or Worker logging changes, reevaluate privacy separately. The current
policy URL `https://ccy.dev/projects/gtmux/privacy` must remain reachable.

## Review Notes template

Based on 1.0.97, with the demo label verified for 1.0.121. Recheck after changes to
pairing/demo entry, camera or push behavior; capture notes are in [assets-ci.md](assets-ci.md).

```
gtmux is a companion app for a server that users run on their own Mac (`gtmux serve`). It shows the user's tmux and coding-agent sessions and lets them reply from their phone. There is no account or login. Users pair with a QR code or code shown on their Mac. Terminal input goes only to the Mac they paired. The app does not offer a VPN, and it collects no data: push notifications are sent by the user's own Mac through a relay that forwards them to Apple's push service without storing them.
REVIEW WITHOUT A SERVER
On the first pairing screen, tap "View demo" ("查看演示" in Simplified Chinese). The self-contained demo uses sample data and needs no credentials or network. It shows the agent radar, terminal and reply controls, HQ, knowledge, usage, and the iPad layout. You can navigate the whole demo without connecting to a server.
Camera access scans the pairing QR code, or takes a photo to send to the user's own Mac. Photo Library access attaches an image to an agent message. Push notifications report agent status.
```
