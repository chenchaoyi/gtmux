## Tasks
- [x] Add read-only doctor JSON/no-prompt contract and meaningful tests.
- [x] Run post-install checks through the new CLI; preserve success/failure boundaries.
- [x] Add menu-bar startup/version state, bounded check runner, grouped UI and interactive repair entry.
- [x] Test relaunch/retry/report parsing, safety and check failure paths.
- [x] Sync English/Chinese docs and main spec; run required gates.

## Acceptance
Synthetic Go and Swift checks cover the new behavior without repairing the real
machine or replacing the running app. Live menu-bar visual/VoiceOver acceptance
and a real update/relaunch remain unverified; no release or installation is part
of this change. Codex raw/fullscreen documentation is source-verified at 0.162.1;
no real client mode was changed. Recovery-package contents were decrypted and
compared file by file; exported calendar reminders have not been scheduled.
