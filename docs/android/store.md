# Android store readiness

[中文](store.zh.md)

The signed APK delivery and the Google Play submission are separate. A build, emulator
pass or encrypted artifact does not create a developer account or publish a listing.
The following requirements were checked against primary sources on 2026-11-03.

| Operator action | Why it is needed |
| --- | --- |
| Register/confirm a Google Play developer account | Google charges a US$25 one-time registration fee and verifies identity/contact information. Choose the real personal or organization identity; organization verification can require D‑U‑N‑S. |
| Complete device/account verification | New personal accounts may need a real Android device through the Play Console app. An emulator does not satisfy that identity step. |
| Recruit real closed-test participants if applicable | Personal accounts created after 2023-11-13 need at least 12 testers continuously opted in for 14 days before applying for production access. Internal testing alone does not replace it. |
| Create/confirm the Firebase project for optional FCM | Register `com.gtmux.app`; keep client configuration separate from the relay's service-account credential. FCM needs Google services; it is not a universal substitute for OEM push. |
| Decide the Play signing identity before uploading | Preserve the direct APK update certificate if the same application ID will use Play. The upload key and Play's app-signing key have different jobs. |
| Complete the candidate-bound listing and disclosures | Prepare phone/tablet screenshots, support/privacy URL, content rating, app-access/demo instructions and Data safety from the actual SDKs and candidate behavior. Do not copy iOS privacy/accessibility declarations. |

The app includes camera/ML Kit and optional Firebase Messaging dependencies. Review
[ML Kit's Play disclosure guidance](https://developers.google.com/ml-kit/android-data-disclosure)
and [Firebase's Android disclosure guidance](https://firebase.google.com/docs/android/play-data-disclosure)
against the actual dependency graph and enabled services. Camera permission is requested
at use; notifications remain optional. Do not declare that a third-party SDK collects
nothing merely because Firebase Analytics is absent.

Validate 16 KB native-library/page support in the produced APK/AAB, including upstream
native dependencies. Google's current [page-size guidance](https://developer.android.com/guide/practices/page-sizes)
sets the applicable Play timing; a package verifier does not replace Console validation.

Android developer verification also affects distribution outside Play. The initial
2026 rollout covers certified devices in Brazil, Indonesia, Singapore and Thailand,
with wider rollout from 2027. Check the [current program](https://developer.android.com/developer-verification)
for the intended countries and open-source/limited-distribution options. This is not a
claim that all sideloading everywhere needs a Play account.

A domestic-store launch, vendor push and non-Google device coverage need their own
channel/account and real-device assessment. No manufacturer account, legal identity,
filing, payment or tester roster is inferred from an Apple account or an APK build.

Sources: [Play registration](https://support.google.com/googleplay/android-developer/answer/6112435?hl=en),
[personal-account testing](https://support.google.com/googleplay/android-developer/answer/14151465?hl=en),
[account type and verification](https://support.google.com/googleplay/android-developer/answer/13634885),
[Firebase setup](https://firebase.google.com/docs/android/setup),
[Android app signing](https://developer.android.com/studio/publish/app-signing).
