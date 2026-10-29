## Implementation
- [x] Verify frontend connection, root identity, process births, home and private transport.
- [x] Preserve embedded/legacy and excluded launch semantics; integrate managed shell/spawn/HQ/restore.
- [x] Make input and relay replies exact, durable and honest; protect drafts and HQ authority.
- [x] Retire only exact minimal startup observations and retain restore/history.
- [x] Complete regression, deliberate guard mutations, documentation and architecture checks.

## Acceptance
Actual Codex 0.165 shared and `--no-daemon` clients use an isolated synthetic workspace. Earlier historical binaries, real-phone delivery, VoiceOver and general Desktop takeover are separate acceptance boundaries; none is implied by simulated or source-level tests.

Actual verification: default shared clients and an embedded control in one synthetic directory; bare-shell resume and `/new` before the first prompt; busy exact-thread queue accepted then one fixed response; draft refusal unchanged; legacy-hook write coexistence; exit removes active receipt, retains history and produces no external duplicate. Seventeen deliberate Go guard mutations failed at behavior assertions and were restored. Browser replacement regressions failed on both 390/1440 widths before the selected-event fix. General historical binaries, cached subagent-focus gestures, physical phone/VoiceOver, Desktop takeover and backend-writer release are not claimed. Full repository and design checks are recorded separately from actual live acceptance.
