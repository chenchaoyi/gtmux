# Proposal: Core, HQ, and Server Hardening

## Why

Production operation and full-codebase review surfaced durability, latency, and security edge cases across the Go core, HQ wake/supervision pipeline, and remote HTTP/SSE/WebSocket server:

1. `hqnudge` retains terminal `.refused`, `.native-uncertain`, and `.legacy-uncertain` files plus uncertain `nativeIntent` and `.pin` anchors forever, risking the 200-intent / 1,600-pin hard limit, stranding historical colliding slots (`claim-conflict`), and exhausting retries after 20s when a user is typing (`InputDraftPresent`) or answering a question (`InputQuestion`).
2. `events.Append` releases the `events.seq` lock before rotation and append, allowing concurrent writers to commit out-of-order sequences or double-rotate `events.1.jsonl`, while markers, tasks, and outcome tallies use non-atomic writes or racy read-then-remove consumption.
3. `events.ReadSince` and `events.Read` linearly scan `events.1.jsonl` + `events.jsonl` from byte 0 on every poll, `hub.run` executes slow ticks synchronously on the SSE/radar loop, and radar/transcript/tmux hot paths perform avoidable file scans, JSON re-unmarshaling, and unbounded subprocess calls.
4. `GET /api/attach` lacks WebSocket ping/pong liveness deadlines on quiet panes and forks `AttachCursor` every 120ms while idle; `/api/events`, `/api/addresses`, guest `/api/agents`, JSON `POST` handlers, and APNs iOS error handling need tighter revocation, scope, size, and dead-token hygiene; and HQ board/snapshot/codex-frontend/mine/prompt/send paths have size, symlink, locking, and parity gaps.

## What Changes

- Archive aged/excess terminal wake evidence into bounded JSONL segments while retiring uncertain `nativeIntent` and `.pin` files, reconcile aged colliding slots cleanly, and hold `InputDraftPresent`/`InputQuestion` refusals with bounded backoff instead of permanent `.refused` exhaustion.
- Hold an exclusive `events.seq` flock across sequence assignment, size rotation, and `O_APPEND` write; switch markers and `tasks/*.json` to atomic private writes; serialize watermark and task updates with `flock`; and atomically rename `tally.log` under lock during consumption.
- Scan `events.jsonl` backwards from EOF for positive-cursor `ReadSince` and positive-window `Read` before touching `events.1.jsonl`, and short-circuit caught-up `hqworkspace` event cursors.
- Make `OnTally` badge pushes async, run `hub.run` `onSlowTick` in a single-flight goroutine followed by `checkServerMode`, memoize `Collection.Agents()`, optimize `idleSince`/transcript stat caches/native `agy` titles/early `/api/transcript` ETag + struct-level context stripping/cached push status snapshots, and bound `internal/tmux` command timeouts.
- Add WebSocket ping/pong read/write deadlines and read limits on `GET /api/attach`, gate `AttachCursor` on pause, idle quiescence, and `predict=0`, re-check SSE tokens per event, restrict `/api/addresses` to full scope, strip `cwd`/`icon`/`launch_host` for guests, bound JSON POST bodies and push token lengths, retire dead iOS APNs tokens (`410` / `400 BadDeviceToken`) and hash oversized native collapse IDs.
- Warn on oversized HQ `board.md` (>32 KiB or >250 lines), cap handoff entries to 10 in playbook v70, truncate `Board()` on UTF-8 rune boundaries, validate snapshot `.json` receipts without double archive decompression, accept and reap `codex-frontends` symlinks/retired records, log `hqworkspace` op errors, lock/bound/back off `mine`, normalize `prompt` labels and choosers, and bring `/api/send` parity (`MarkDelivered`, non-user `AuditSend`, combined pane metadata query).

## Surfaces

- **terminal**: Faster CLI `events`/`agents`/`digest` reads, bounded `tmux` subprocess timeouts, `gtmux attach` `predict=0` cursor gating and dead-connection cleanup, accurate `gtmux doctor` board/symlink checks, and reliable HQ wake recovery.
- **menubar**: Non-blocking `gtmux serve` radar/SSE loop and faster loopback `/api/agents` & `/api/hq/board` responses consumed by `Gtmux.app`.
- **phone**: Responsive SSE heartbeats, early `304` on `/api/transcript`, single-query `/api/pane` & `/api/send` metadata, task-delivery marking on `/api/send`, dead iOS APNs token retirement, and guest path redaction.
- **iPad**: Same HTTP/SSE responsiveness, conditional transcript revalidation, `/api/send` delivery parity, and APNs token hygiene as phone.
- **Web**: Prompt SSE disconnect on token revocation or link expiry, `403` guest enforcement on `/api/addresses`, host-path/icon stripping on guest `/api/agents`, and bounded JSON POST bodies.
