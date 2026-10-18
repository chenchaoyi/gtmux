# DeepSeek Harness Web conversations

## Why

Owners already use a running `dsh web` instance and need its conversations visible alongside verified Codex desktop conversations. Treating the Web server's tmux pane as one agent would lose individual sessions and imply controls that the pane cannot safely provide.

## What changes

- Register DeepSeek Harness once and use its official shipped mark, with source provenance.
- Add a read-only, version-aware DSH session reader for the installed 0.2.0-rc.2 v4 format: raw JSONL and independently checksummed Zstandard frames, exact header identity, configured-home isolation, bounded incremental reads and explicit unsupported/corrupt-state handling.
- Discover user conversations without requiring a restarted Web server or modifying DSH credentials/profile files. Keep subagent sessions separate and distinguish stored activity from verified live control.
- Share an external-conversation provider interface with existing Codex readers. Preserve existing Codex APIs and home-scoped grants while adding DSH identity, public transcript, title, state and source-agent fields.
- Reuse per-conversation HQ follow, notifications and knowledge consent. Discovery and user-initiated reading do not grant any of them. Keep guests outside external-conversation readers and settings.
- Install a dedicated native Cordis overlay/plugin for future lifecycle and question events, without Codex compatibility hooks or approval decisions. Preserve user profiles and leave the current Web server running.
- Provide public conversation readers in the CLI, native Mac app, phone, iPad and Web. Poll safely, retain validators, discard stale responses and use the existing transcript budget.

## Surfaces

- 终端 (terminal / attach): `agents`, `digest`, `transcript` and `follow` expose DSH identity and read-only content; no DSH TUI control or move-to-tmux claim is added.
- 菜单栏 (menubar): native external-conversation rows, official icon, readable history and existing per-conversation permission controls.
- 手机 (phone): DSH Web conversations open the common read-only reader with the right agent identity and home scope.
- iPad: the same reader and settings in the existing regular-width shell.
- Web: external conversation rows can read public history; permission handling and request lifecycle follow the existing server/browser conventions.

## Evidence and limits

The installed DSH is 0.2.0-rc.2 and writes `session.v4.jsonl.zstd`. Logs contain a session header and typed `user/message`, `assistant/message`, tool, title and turn lifecycle events. Each appended Zstandard frame is independently decodable; a single one-shot Node decode reads only the first frame. The session lock is a kernel write lease, not a JSON status record. Persistence identity does not prove that a particular UI currently owns a session. DSH Web is already listening on loopback; inspection must not restart it or rewrite its composed profile. Model reasoning, injected system context and credentials are not public chat output.
