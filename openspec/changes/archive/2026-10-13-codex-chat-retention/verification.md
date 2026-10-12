# Verification — 2026-10-13

- Reported Codex rollout: approximately 2.08 GiB. Read-only diagnosis found 295
  raw user message records; instruction/context filtering means these are not
  all distinct user questions. No private content is included in the repository.
- New reader: 274 parsed turns, all with prompts. Applying the existing 512 KiB
  response budget retained 137 recent turns. First read took about 275 ms;
  unchanged cached read about 0.3 ms on this Mac with warm filesystem caches.
  These are local measurements, not cold-disk performance guarantees.
- Synthetic regression: 65 MiB tool output inside a user turn preserves both
  preceding prompts and replies. Further tests cover partial record completion,
  append-only offsets, immutable previous snapshots, tool append, retained turn
  cap, rewrite/replacement, reordered headers and propagated read errors.
- `make check`: passed (format, vet, staticcheck, all Go race tests, 32 publisher tests).
- `npm run check`: passed (TypeScript, ESLint, 154 Jest suites / 1640 tests).
  Whole-suite Jest reported a forced worker exit/timer warning; targeted Chat
  history tests with `--detectOpenHandles --runInBand` passed and exited cleanly
  (4 tests). This change does not claim to resolve the suite-wide teardown warning.
- OpenSpec strict change validation: passed.
- No native app build, install, App Store upload, live session mutation, real-device
  layout or VoiceOver acceptance was performed. UI collapse behavior is tested
  with React rendering; the functional fix is in the Mac/server Go reader.
- `scripts/check-design.sh`: passed after OpenSpec sync/archive (including cgo-free build and strict spec validation).
