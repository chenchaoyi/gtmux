# Capture App Store images in Actions

When local disk space is limited, run **App Store screenshots (maintainers)**
(`ios-store-assets.yml`) from `main`, with a published stable `release_tag`.
The workflow verifies the release and successful main CI, then builds that exact
source for Simulator. It needs no signing, publisher or App Store credentials.

It reuses the existing demo capture and framing scripts. A dedicated iPhone 17 Pro
Max and iPad Pro 13-inch simulator each capture English and Chinese. Only simulators
created by the run are removed. The artifact contains 22 framed screenshots, two
creative images, the raw demo captures and a manifest of source IDs and image hashes.
The lock-screen image remains a rendering of the widget, not a real push test.

```sh
gh workflow run ios-store-assets.yml --ref main -f release_tag=vX.Y.Z
```

Download the resulting `appstore-assets-<tag>-<run>` artifact privately. Inspect every
image at its original size. Copy `screenshots/<locale>/` and `creative/<locale>/` to
the corresponding `mobileapp/fastlane/` folders; retain the manifest with release
evidence. Follow [the submission runbook](submit.md) to place and read back all four
screenshot groups and the four Header/Search Results placements. This workflow
does not upload to App Store Connect or submit a version for review. Its images do
not establish real-device, VoiceOver, APNs or Live Activity acceptance.
