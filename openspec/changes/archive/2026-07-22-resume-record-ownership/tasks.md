# resume-record-ownership — tasks

- [x] 1.1 Guard the resume-record write: only a parseable conversation may claim a pane it doesn't own
- [x] 1.2 Keep the cheap paths cheap — no-incumbent and same-session decided without reading a log
- [x] 1.3 A vanished incumbent never pins the pane
- [x] 1.4 Tests: stub refused · handover allowed · same-session update · dead incumbent replaceable · unresolvable agent reads as dead
- [x] 1.5 `make check` + `check-design.sh` green
- [x] 1.6 Repair the already-clobbered record on the reporter's machine (it had self-healed on the next hook — the clobber is intermittent, which is why chat was blank only sometimes)
- [x] 1.7 sync-specs + archive
