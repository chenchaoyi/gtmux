## Implementation

- [x] Add bounded master-only paired-device label mutation, preserving identity and scope.
- [x] Add Mac label editing with explicit save/cancel and failure feedback.
- [x] Correct generic labels, client metadata capture and browser/phone icons.
- [x] Unify Web mode order and inconsistent status accessibility labels.
- [x] Add regression tests and synchronize contract, spec and bilingual docs.
- [x] Run core, mobile, Swift and design checks and scoped rendered Web validation.

## Acceptance

No device builds or installations planned. Physical iPhone, iPad and VoiceOver checks remain separate from source/component tests. Browser metadata is best-effort and coarse; a user-assigned iPhone name is not available without an Apple-granted entitlement. Rendered Web validation used isolated fixture data at desktop and 390px widths: Chat/Terminal switching, plain-pane mode, saved-mode retention and no console warnings/errors. Mac name editor fits both languages and appearances in offscreen native rendering. Mobile checks pass; the normal full suite emits a transient exit warning, while a detectOpenHandles run exits without identifying leaked handles. This does not establish device or spoken VoiceOver acceptance.
