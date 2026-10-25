## Implementation

- [x] Pin the snapshot contract and failure/freshness rules with meaningful tests.
- [x] Add shared adaptive usage views, provider selection and timeline boundaries.
- [x] Wire iOS parent-only refresh, safe context lifecycle, App Group and deep link.
- [x] Evaluate native macOS packaging and document its provisioning/embedding prerequisites; share the compiled model and views without advertising a native extension.
- [x] Run exploratory App/iPad/macOS/Web checks and fix evidenced high-value findings, including fixed-control text clipping at accessibility sizes.
- [x] Update paired documentation and sync the implemented specification.

## Acceptance

Author self-testing, not independent review: iOS app/extensions compile; actual Phone Home widget selection and large-text layout; native empty/old/reset/long/missing-provider previews; parent refresh races; iPad portrait/landscape and usage without HQ; Web narrow/wide, long content and polling; shared native macOS view rendering. Tests and a standalone simulator fixture accompany the implementation.

Unverified: physical devices, VoiceOver speech, real Lock Screen installation, App Group capability registration and signed provisioning profiles, actual system background refresh budgets. macOS native widget packaging is explicitly deferred; shared views are not a shipped Mac extension. The previously submitted Store build is unchanged. Simulator and snapshot checks do not establish these results.
