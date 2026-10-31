## Implementation
- [x] Unify matter counts and keep the workspace snapshot current in the conversation.
- [x] Remove duplicate modern attention tabs/inspector and retain labelled old-server fallback.
- [x] Fold the whole context with compact critical state and align HQ identity.
- [x] Cover authority, navigation, disclosure and state preservation regressions.
- [x] Verify native phone/iPad layouts and document evidence limits.
- [x] Sync canonical specifications/design/user docs and archive.

## Acceptance
Current components render in an independent Release fixture on the existing iPhone/iPad simulators. Phone light/dark, collapsed/expanded, resource pressure and Dynamic Type layouts are inspected. React action regressions cover navigation, folding, current shared counts, legacy fallback, capability recovery and authorization invalidation. The simulator runtime snapshot returns one root and no targets: native tapping, scrolling, VoiceOver and physical-device acceptance remain unverified. No new release or user-device installation is included.
