# Tasks

- [x] Inspect exact released source and available refusal metadata.
- [x] Commit reviewed, sealed test design before test implementation.
- [x] Implement independent document ownership, unique send files and bounded lifecycle.
- [x] Implement searchable session picker, keyboard/IME handling and bilingual copy.
- [x] Verify truthful refusal categories and content-free diagnostics.
- [x] Run old-code negative controls, affected synthetic/native/IPC regressions and normal source/core checks.
- [ ] After the authorized App update, manually verify multiple real screenshot windows and target picking; no screenshot may be sent to a real session without an explicit user action.

Catalogue retirement: `testRefusedWaitingFromTheCLIMapsToWaiting` is replaced by `testRefusedWaitingWithoutEvidenceStaysUnconfirmed`. A bare legacy state no longer proves a human request. Original approval, before/after-paste, unknown-result and receipt consistency assertions remain; no duplicate stale selector is retained.
