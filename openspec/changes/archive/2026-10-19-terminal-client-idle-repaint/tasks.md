## Implementation

- [x] Declare native lifecycle observation in the client adapter capability.
- [x] Publish bounded private lifecycle snapshots only from verified native hook senders and accepted turns.
- [x] Prefer current native phase over frame/CPU and inferred markers; recover missed hooks only with later explicit matching DSH boundaries; retain unknown as neutral.
- [x] Verify real starts/questions/resumes/outcomes, stale callbacks, binding changes, owned bounded records and orphan cleanup.
- [x] Prove test sensitivity by deliberate regressions, run repository gates and update bilingual documentation.
- [x] Archive this implemented change in the finishing PR.

## Acceptance

Live validation remains read-only: no model prompt or synthetic user input is sent to the user's conversation. Author tests and self-review are not independent review. Physical device and VoiceOver acceptance are separate from shared core status validation.
