fastlane documentation
----

# Installation

Make sure you have the latest version of the Xcode command line tools installed:

```sh
xcode-select --install
```

For _fastlane_ installation instructions, see [Installing _fastlane_](https://docs.fastlane.tools/#installing-fastlane)

# Available Actions

## iOS

### ios verify

```sh
[bundle exec] fastlane ios verify
```

Quick ASC auth check — no build. Proves the API key + env are wired.

### ios release

```sh
[bundle exec] fastlane ios release
```

Build + sign + archive + upload the binary to App Store Connect (auto build number)

### ios upload

```sh
[bundle exec] fastlane ios upload
```

Upload an already-built ipa to ASC (no build). Pass ipa:<path> (default build/GtmuxMobile.ipa).

### ios metadata

```sh
[bundle exec] fastlane ios metadata
```

Upload App Store listing text + screenshots (en-US + zh-Hans). No binary. Pass version:<x.y.z> to override.

----

This README.md is auto-generated and will be re-generated every time [_fastlane_](https://fastlane.tools) is run.

More information about _fastlane_ can be found on [fastlane.tools](https://fastlane.tools).

The documentation of _fastlane_ can be found on [docs.fastlane.tools](https://docs.fastlane.tools).
