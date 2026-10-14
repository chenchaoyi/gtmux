## Core
- [x] Add transactional workspace models, explicit association intervals and audit history.
- [x] Add scoped messages, idempotent receipts, safe hold/claim/uncertain/retry behavior.
- [x] Compose existing core readers into bounded CLI workspace/detail/activity reads.
- [x] Carry explicit matter identity through spawn and structured HQ update protocol.
- [x] Cover recycled identity, association changes, merge/split, failures and concurrent requests.

## Menu bar
- [x] Add modular client/store/window and stable native sidebar selection.
- [x] Add New task/global conversation, scoped drafts and left/right conversation/avatar.
- [x] Add participant navigation/management and bounded source/evidence disclosure.
- [x] Add activity view and reuse board/knowledge/usage doors and terminal escape hatch.
- [x] Verify loading/error/retry, old-core compatibility, scroll preservation and accessibility.

## Delivery
- [x] Synchronize bilingual docs, CLI help, capability specs and playbook instructions.
- [x] Run meaningful targeted tests, deliberate regression checks, make check and design gate.
- [x] Run Swift tests/build and native layout/visual checks within resource budget.
- [x] Sync and archive this change in the same delivered PR; report acceptance limitations.

## Acceptance limits

Tests use isolated stores and fake clients; no real HQ instruction, rotation or
personal-record migration was performed. CUA approval was requested because auto-review
rejected starting a locally built preview; the user explicitly approved that run.
In the isolated native window, sidebar selection, left/right layout, Return newline,
Command-Return rejection feedback, draft retention and per-scope drafts were checked.
This caught and fixed the initial-window shrink and SwiftUI Return-commit behavior.
Opening conversation management interrupted the native automation pipe. The preview
process remained running and no preview crash report was present; management-modal,
source-sheet and VoiceOver runtime acceptance remain unverified. Native hosting/logic
tests are separate evidence. No release or installation is included.

Deliberately reverting the canonical delivery scope fails its test. Removing the
saved-session match allows a recycled pane to steal membership and fails its test.
Correct source was restored after both experiments. Disk began at about 13 GiB;
builds used one owned Swift scratch and existing Go caches, with no mobile/simulator
build. Temporary app uses its own bundle ID and rejects every production command.

Final integration checks cover the shared input lease across workspace delivery,
automatic wakes, rotation and startup briefing. Removing the lease permits the
simulated wake/startup to type and fails the tests; removing rotation's durable
attempt refresh repeats the reset and fails its test. Reopening the isolated preview
still failed native automation with `cgWindowNotFound`, so source/management modal
and VoiceOver acceptance are not claimed. The installed application was untouched.
