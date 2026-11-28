# Design: Core, HQ, and Server Hardening

## 1. HQ Nudge Archival, Slot Reconciliation, and Draft Hold (`internal/hqnudge`)

- Introduce `terminalArchivePath()` (`hqwake/terminal-archive.jsonl`, 2 MiB rotation to `.1` under lease) recording archived terminal files (`.refused`, `.native-uncertain`, `.legacy-uncertain`) and retired `uncertain` `nativeIntent` records.
- Retain at most 32 recent terminal wake files within 24h on disk; archive older or excess entries and retire their corresponding `nativeIntent` JSON + `.pin` hard-links so `hq-nudge-native-batches/` and `hq-nudge-native-claims/` never hit the 200 / 1,600 recovery caps.
- When settling a valid claim (`X.sending`) or orphan claim collides with an aged (`>= 60s`) distinct regular file at `X.<retained>`, archive the older retained file (and any matching `uncertain` `nativeIntent` + `.pin`) with both SHA-256 digests first, unlink `X.<retained>`, and complete parking so `protected` clears and `noteCleanRecovery` resolves `claim-conflict`. Fresh (<60s) collisions remain protected in-place.
- In `setNativeWakeOutcome`, when refusal is `InputDraftPresent` or `InputQuestion`, keep `Outcome = "refused"` with bounded backoff capped below `nativeAttemptLimit` (clamping `Retry.Number` at `nativeAttemptLimit - 1` with 15s delay) rather than transitioning to permanent `.refused` (`Outcome = "held"`).

## 2. Atomic Event Append, Tail Reads, and State/Task/Tally Serialization

- Acquire an exclusive `flock` on `events.seq` for the entire `appendWithCap` critical section (`nextSeqLocked` + `rotateIfNeeded` + `O_APPEND` write), guaranteeing strictly ordered `Seq` lines on disk and preventing double rotation.
- Implement backwards chunked scanning from EOF of `events.jsonl` in `ReadSince(cursor > 0)` and `Read(sinceSecs > 0)`, falling back to `events.1.jsonl` only when `events.jsonl` is absent/empty or its earliest scanned record has not yet crossed `Seq <= cursor` or `Ts < cutoff`. Short-circuit `ingestWorkspaceObservations` when `cursor >= events.CurrentSeq()`.
- Route `state.WriteMarker`, `state.WriteLastFinished`, and `dispatch.AddTask` through `state.WritePrivateFile` (temp+rename). Distinguish `os.ErrNotExist` (`WatermarkUnset`) from empty/corrupt (`0`) in `hqwake.Consumed()`, serialize `Consume` and `dispatch` task mutations (`updateTask`, `SnoozeTask`) under advisory `flock`s, and atomically rename `tally.log` under lock in `consumeTally()`.

## 3. Non-Blocking Hub, Radar/Transcript Fast Paths, Attach Liveness, and HTTP/Push Security

- Dispatch `pushBadge` asynchronously in `PushManager.OnTally`. Guard `hub.run`'s `onSlowTick` + `checkServerMode` pair with an `atomic.Bool` single-flight goroutine (serializing `lastServerModeSig` safely) so slow subprocesses never stall `1.5s` radar ticks, `3s` fast ticks, or `20s` SSE pings.
- Memoize `panes []Pane` inside `radar.Collection.Agents()`, return `marked` early for non-Codex panes in `idleSince`, reuse memoized `capture` in `agents.go`, use `transcript.AgySessionTitle` for native `agy` rows, cache `claudeLogPath`/`LastMessageError`/`SessionTitles("codex")` by `(dev, ino, size, mtimeNano)`, short-circuit `/api/transcript` on matching `If-None-Match` while stripping `Context` on `[]transcript.Turn` structs, wire `PushManager.SetStatusSource` to a TTL-cached snapshot reader, and enforce a 5s default command timeout in `internal/tmux`.
- Add WebSocket `SetReadLimit(64<<10)`, `SetReadDeadline` refreshed by reads/pongs, periodic `PingMessage` writes under `wmu` with `SetWriteDeadline`, and gate `AttachCursor` on `!flow.paused()`, 2s activity window, and `predict != "0"` (`connect` appends `&predict=0` when `!predict`).
- Re-verify enrolled/guest `TokenScope` before writing SSE frames in `handleEvents`; enforce `fullOnly` on `/api/addresses`; delete `cwd`, `icon`, and `launch_host` in `filterAgentsForGuest`; wrap 8 JSON `POST` handlers in `http.MaxBytesReader(..., 64<<10)` and cap push tokens at `<= 512` bytes; parse APNs `detail.reason` to retire iOS tokens on `410` or `400 BadDeviceToken`; and SHA-256 hash `native:` collapse IDs exceeding 64 bytes.

## 4. HQ Board, Snapshots, Codex Frontends, Mine, Prompt, and `/api/send` Parity

- Truncate `hq.Board()` on a valid UTF-8 rune boundary, flag boards exceeding `32 KiB` or `250` lines in `boardsize.go`/`doctor.go`/`selfcheck.go`, and update playbook v70 (`en` + `zh`) to keep at most 10 handoff entries.
- Validate `snapshotReceipt` + file size/mtime in `managedSnapshotPoints()` without decompressing `.tar.gz` archives during listing (full archive verification remains on the newly written candidate).
- Accept owner-created `codex-frontends/client-*/frontend.sock` symlinks targeting `/tmp/codex-daemon-<uid>/` in `InspectPrivateTree`; reap dead `client-*` dirs (>60s old with `ECONNREFUSED`/`ENOENT`) and retired/dead-PID `resume/codex-frontends/*.json` files in `diskHygieneSweep`; and ignore `Retired` records before applying the 128-active-entry cap in `resume`.
- Log `"error", err` (using `diag.Refused` for expected conflict/not-found/validation refusals) in `hqworkspace.go`; add lease locking, `state.WritePrivateFile`, `emitted.json` cap, `last-pass` marker, and error backoff to `mine`; normalize `agent` in `bootBannerLine` and scan all registered startup gates/pickers in `looksLikeStartupChooser`; and add `dispatch.MarkDelivered`, non-user `events.AuditSend`, and single-call pane width+cursor lookup to `serve.go`.
