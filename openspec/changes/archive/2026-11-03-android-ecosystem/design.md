# Design and acceptance

## Baseline
The existing Android directory is a React Native template. The Android implementation shares business state, navigation and readers with iOS, with native adapters for OS facilities. Android has a separate build number and signing identity, while the semantic version follows the shared mobile source.

## Architecture
Business state and JSX remain shared. Kotlin implements the existing native bridge contracts, using bounded native HTTP streams, no credential in DOM and no arbitrary file access. Go retains artifact/session/home/parent authorization. Standard platform parsers may be adapters; do not hand-port a second Markdown/security model. Share presentation assets/style/policy fixtures wherever possible and preserve current iOS fallback behavior. Document preview needs one clear dismiss action, Android Back, local HTML/Markdown images and explicit script consent; no general JavaScript bridge, redirect bearer forwarding or world-readable cache.

Provider notifications are typed (APNs versus FCM), owner-scoped and opt-in. Missing Firebase/relay setup or unsupported Google services must be visible; never report a token fixture as live delivery. Android notifications open the current session; they do not send numeric approvals from background content. Existing iOS quick replies retain their current behavior. Native widgets consume bounded credential-free display snapshots; parent owns network credentials, lifecycle and guest clearing.

Debug/QA is distinct from release. Release cannot silently use the public debug keystore. Version name, monotonic version code, source SHA, JS/native mapping and actual package certificate are recorded. Existing signer identity/profile/device sets for iOS are untouched. Android signing/recovery is private and separately budgeted; no private key/service account in APK, Git or public artifacts.

## QA
Own emulator AVD/data/ports/ADBserials only, one at a time to contain memory. Reuse installed base images and compatible caches, do not reset globalSDK/ADB or other simulators. Phone/tablet/window-resize, day/night/system, large text, Back/IME/hardware keys, owner/guest, pair/unpair/switch, offline/reconnect, Chat/Terminal/HQ/Trace pagination/folds/tool input and code/diff readers, documents/embeddedimages/scripts/share, notifications and widget states all need evidence. Distinguish unit checks, real emulator taps, screenshots, OS permission behavior and actual FCM/store acceptance. Failed/skipped tests are recorded, not counted as passes.

## Release and store
Unsigned/QA native builds run without signing secrets. Trusted-main delivery builds production-mode unsigned APK and AAB, validates source/native payloads and encrypts the result to a supplied public recipient. Local signing requires the intact exact-source inventory and the private application key, followed by independent package/bundle verification. Private keys do not enter the cloud workflow. Test publication retains scoped access and an encrypted recoverable result. Account identity/payment, Play verification and actual Store publication require genuine operator/account evidence. Current official prerequisites include personal-account testing/device verification and the 2026 Android developer verification rollout; report concrete actions/costs with primary references rather than assuming an account exists.

## Storage
Declare a task size budget and minimum free-space floor before heavy writes; measure actual bytes independently. Existing SDKs/caches protected. APK/native intermediate trees remain task-local; only unique signing/mapping/recovery and small acceptance records are retained under declared budgets. Review dates do not authorize automatic deletion.
