## 1. Event Journal, State, and HQ Wake Hardening

- [x] 1.1 Archive aged/excess terminal wake evidence, retire uncertain `nativeIntent` + `.pin` anchors, reconcile aged colliding slots, and hold `InputDraftPresent`/`InputQuestion` refusals in `internal/hqnudge`
- [x] 1.2 Serialize `events.Append` sequence/rotation/write under one `flock`, add tail-first `ReadSince`/`Read` + `hqworkspace` cursor short-circuit, and harden atomic markers, watermarks, tasks, and `tally.log`

## 2. Hub, Radar, Transcript, Attach, and HTTP/Push Hardening

- [x] 2.1 Make `OnTally` badge pushes async, run `hub.run` `onSlowTick` in a single-flight goroutine with `checkServerMode`, and memoize `Collection.Agents()`
- [x] 2.2 Optimize `idleSince`, transcript stat caches, native `agy` titles, conditional `/api/transcript` + struct context stripping, cached push status snapshots, and `tmux` command timeouts
- [x] 2.3 Add `GET /api/attach` ping/pong deadlines + cursor gating, SSE token re-checks, `/api/addresses` full-only gate, guest field redaction, `MaxBytesReader` + token caps, and APNs dead-token/collapse-ID handling

## 3. HQ Board, Snapshots, Codex Frontends, Mine, Prompt, and Serve Parity

- [x] 3.1 Add HQ `board.md` size/line warnings, UTF-8 truncation, playbook v70 handoff cap, and receipt-only snapshot listing
- [x] 3.2 Handle `codex-frontends` socket symlinks and dead/retired reaping, `hqworkspace` error logging, `mine` locking/bounds/backoff, `prompt` normalization, and `/api/send` parity + unit tests
