# Codex terminal identity

**English** · [中文](codex-terminal-identity.zh.md)

Shared backend identity differs from terminal-client identity. Codex 0.156 added optional automatic background operation; 0.157 enabled it by default for eligible TUI launches. Native hooks still identify a conversation with `session_id`; a subagent can supply its parent session plus `agent_id`. The backend PID and inherited `TMUX_PANE` therefore cannot identify the frontend. See official [0.156 notes](https://github.com/openai/codex/releases/tag/rust-v0.156.0), [0.157 notes](https://github.com/openai/codex/releases/tag/rust-v0.157.0) and [hook fields](https://learn.chatgpt.com/docs/hooks#common-input-fields).

## Attachment evidence

`internal/codexbridge` forwards one eligible frontend's local WebSocket connection through a private Unix socket. It verifies same-user kernel peer credentials, actual Codex foreground PID and birth, controlling terminal, pane incarnation, backend PID/birth and canonical Codex home. Only its own successful root `thread/start`, `thread/resume` or `thread/fork` response updates identity. Failed requests, global notifications, loaded/history lists, ephemeral title helpers and subagent views cannot claim a pane. Sequence checks prevent late older requests/connections from replacing a newer root. `/new` establishes the new root before the first prompt.

The private rendezvous follows Codex's protected local-daemon layout. Existing default sockets, permissions and access settings are preserved. Frames are forwarded unchanged with a 64 MiB bound. Only identity requests enter a bounded pending map; protocol bodies are neither logged nor stored by the bridge. A measured 0.165 startup catalog response was about 24 MB; the small observation limit must not become the forwarding limit. This catalog is transport data, not proof of model-context use.

Private metadata receipts heartbeat once per second and expire after five seconds. Input also rechecks foreground process, terminal, backend and root identity. Exiting removes only the active receipt/socket; saved resume history remains. An unchanged minimal native SessionStart may be retired only with exact frontend/backend proof and a generation-checked receipt. Actual native turns and later independent activity survive. A live shared daemon is never stopped to clean up one client.

## Launch and compatibility

`gtmux codex -- …` admits supported interactive launches by capabilities, not version strings. Standard spawn, HQ and restore use the same adapter. The hook installer and doctor offer an owned Bash/zsh block for new interactive tmux shells; existing aliases/functions are preserved, and uninstall removes only that block. Existing running clients are not restarted or retroactively assigned fabricated receipts.

Older independent clients retain the original foreground-process/hook route. Explicit `--no-daemon`, remote, profile/config overrides, headless, unknown options and custom executable wrappers retain native arguments, environment and exit behavior. They do not receive an unverified bridge binding. Eligible cold interactive launches may call only official `codex app-server daemon start`; list/history reads never start or resume anything. Failure before starting a child falls back once to the original executable. Once started, a child is never automatically launched a second time.

Implementation evidence is pinned to official `rust-v0.165.0`, commit `360319d8dee62d87e2d59fc0c21805d79519cfed`: `codex-rs/tui/src/daemon_startup.rs`, `codex-rs/tui/src/app_server_connection.rs`, `codex-rs/app-server-protocol/src/protocol/v2.rs` and `codex-rs/app-server-transport/src/uds/daemon_directory.rs`. Capability failure is a safe native fallback, not a claim that future protocols are supported.

## Input, HQ and replies

A verified bridged frontend uses addressed `thread/queue/add` with exact home/thread and expected backend incarnation. Existing draft, waiting-prompt, target, HQ-input-lease and hold guards still run. Text and authorized upload images retain their input kinds. The shared durable input spool claims an attempt exclusively before mutation. Matching client-message and queue IDs establish acceptance; queue acceptance does not establish execution. Ambiguous writes remain uncertain and never fall back to terminal paste or automatic replay. A completed same-ID attempt returns its existing receipt. An explicitly acknowledged uncertain retry uses a new transport generation.

Agent tool commands resolve `CODEX_THREAD_ID` (legacy `CODEX_SESSION_ID` fallback) against canonical home and current bindings. An inherited pane, cwd or subagent parent cannot grant HQ role. Reports retain exact thread/home/process identity; thread-only reports are allowed without inventing a pane. HQ authorization, event consumption, echo suppression and CLI audit attribution share this resolver. Relay replies use a verified independent legacy process route or exact native thread route. Changed/unavailable sources keep replies pull-readable in the ledger; an uncertain native reply is not retried blindly.

All surfaces consume the same Go evidence. Menubar pending actions use one caption font and concise Retry/Cancel send labels. Browser, phone and iPad keep existing input authorization and receive queued/refused/uncertain outcomes. No UI infers ownership from a logo, originator, title or directory.

## Verification boundaries

Synthetic fixtures cover helpers, reordered roots, reconnect sequence, PID reuse, homes, foreign hooks, expiry, old launch behavior, shell overrides, queue receipts, durable deduplication, uncertain writes, draft/hold guards and worker/HQ identity. Actual 0.165 shared clients and an explicit embedded `--no-daemon` control validate the two current architectures. Historical binaries, every custom wrapper, cached subagent-focus UI, real-phone delivery and VoiceOver require separate acceptance; protocol-root tests do not imply those physical checks. General Desktop takeover remains unavailable. This change neither archives history nor changes remote access policy.

HQ wake batches use the same guarded addressed route for managed Codex. Uncertain native batches retain their original entries separately for reconciliation and degradation reporting; they cannot be coalesced into a new message ID and replayed. Other agents retain the existing terminal wake protocol.

The launch preserves explicit `-C`/`--cd`, supplying the original terminal cwd only when absent. The root response workspace is stored separately from the supervising shell cwd, so resume and `/new` do not misroute hooks. A verified hook also checks the captured backend birth. Codex hooks carrying `agent_id` are subagent events: they cannot overwrite the parent profile or complete/wake its root turn.

Browser Trace preserves selected text only while the same event exists. Replacing a session immediately replaces a removed event's detail, including on narrow screens; it cannot leave the prior session visible under the new list.
