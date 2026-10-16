# Reuse a verified release binary for store preparation

The normal path is `fastlane release`, which archives, exports an App Store IPA and
uploads it. Keep the archive and matching dSYMs. With limited local disk space, a
maintainer may reuse an independently verified Release IPA from the exact published
source. An Ad Hoc IPA must be re-signed for App Store distribution before upload.
Re-signing cannot recover an archive or missing dSYMs from an IPA.

Work on a private copy. Reuse the existing Apple Distribution identity and valid
App Store profiles for the main app, widget and notification extension. Pick the
next unused ASC build number and apply it to all three bundles; preserve the public
version and bundle identifiers. No new certificate is needed when the existing
identity is valid and its private key is available.

Use [fastlane's `resign`](https://docs.fastlane.tools/actions/resign/) with
`use_app_entitlements: true`. Its default mode copies the profile's entitlements,
including generic keychain groups, instead of preserving the app's actual rights.
Do not accept that as equivalent to a normal Xcode export.

Before upload, independently check:

- All three signatures are valid, all versions/builds match, and the profiles are
  unexpired App Store profiles without registered-device or enterprise limits.
- Read the signed DER entitlements. Compare them with the original verified IPA;
  for this app, only enabling `beta-reports-active` may differ. Keychain/app groups,
  application/team identifiers and other runtime rights must retain their values.
  `get-task-allow` stays false and the main app's APNs environment stays production.
- The bundled JavaScript hash and native executable UUIDs match the original IPA.
  The app supports both device families and does not require full screen on iPad.

Use the existing `fastlane upload` lane only after those checks. Apple's `VALID`
processing result does not establish entitlement continuity or retained pairing.
Attach the exact processed build with `asc-attach-build.rb`, then read the
association back. A superseded upload stays unselected; do not silently submit it.
Follow the [submission runbook](submit.md) for metadata and assets, and retain the
operator's final Submit step when requested.
