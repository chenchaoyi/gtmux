## Implementation
- [x] Add streaming Codex record filtering and bounded turn retention.
- [x] Add append-only incremental reads with partial-record handling and immutable snapshots.
- [x] Verify large tool output, incremental growth, replacement and retention regressions.
- [x] Verify the reported rollout with counts and timings only; do not commit private content.
- [x] Update English/Chinese documentation and run required checks.

## Acceptance boundaries
No real-device layout or VoiceOver acceptance is claimed. The fix is in the shared
Go transcript reader. Existing UI collapse behavior remains one preview per turn.
