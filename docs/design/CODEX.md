# Codex integration

This is the map of Codex-specific behavior. See [agent onboarding](agent-onboarding.md)
for the common agent contract and [CLI](../cli.md) for commands. Keep this page and its
Chinese pair aligned when a Codex release changes its hooks, screen, or rollout log.

| Concern | Rule | Implementation |
|---|---|---|
| Identity and history | The registry defines Codex's process, resume command, hooks, parser, and icon. Each transcript turn retains its source agent; switching HQ from Claude to Codex does not relabel old turns. | `internal/agents/registry.go`, `internal/transcript/transcript.go`, `mobileapp/src/ui/ChatView.tsx` |
| Startup | Directory trust, hook review, and an MCP startup line are not a ready composer. A previous Claude screen left behind during an in-place switch cannot make Codex ready. | `internal/prompt/prompt.go`, `internal/prompt/prompt_codex_test.go` |
| Hooks | `gtmux install hooks --agent codex` adds `~/.codex/hooks.json` entries and enables `features.hooks`, preserving an existing legacy `notify`. New hooks may need a one-time trust choice. Restart running Codex processes after a hook change. | `internal/app/codex_hooks.go`, `internal/hook/classify.go` |
| Attribution | Codex's shared app-server can emit a hook with no cwd or session id and another client's inherited `TMUX_PANE`. Completion needs a unique bound rollout that logged `task_complete`; approval needs a unique bound session. Otherwise leave the event pane-less and let radar inspect the actual pane. | `internal/hook/codexpane.go`, `internal/radar/codexcompletion.go` |
| Task delivery | `[Pasted Content N chars]` confirms only that a paste reached the composer when N matches the payload size. A matching submit event also needs the bound conversation ID. If Enter is swallowed, retry it only while the recorded folded draft remains; never paste a second copy into that draft. | `internal/dispatch/deliver.go`, `internal/dispatchbridge/dispatchbridge.go` |
| Desktop alerts | `PermissionRequest` fires before auto-review; only a persistent numbered menu on the attributed pane means human input. Unattributed completion gets no generic banner. HQ routine completion is silent, while genuine HQ input can notify. Suppression reasons enter structured diagnostics. | `internal/hook/hook.go`, `openspec/specs/notifications/spec.md` |
| Ghostty | Codex TUI can send a terminal escape notification independently of gtmux's menu-bar queue. A **new Codex HQ process** gets `-c 'tui.notifications=["approval-requested","plan-mode-prompt"]'` unless its command sets the option. Ordinary Codex keeps the user's settings. `gtmux hq --rotate` queues `/new` for delivery after the current turn, **inside the same process**; it cannot update launch flags. Exit the process and run `gtmux hq` to apply them. | `internal/hq/hqagent.go`, `internal/hq/rotate_pending.go` |
| Chat | Codex rollout JSONL uses `response_item` user messages with `input_text` in current versions; older logs may use `event_msg.user_message`. Injected instructions and environment are excluded. Unknown records do not hide later turns. | `internal/transcript/codex.go`, `internal/transcript/testdata/codex-current.jsonl` |
| Outside tmux | `source: native` means no tmux pane; it does not imply a terminal window. A matching rollout's `session_meta.originator` distinguishes ChatGPT desktop (`codex_work_desktop`) from terminal Codex (`codex-tui`) as an additive `client` field. The rollout `source` value can be `vscode` for either, so it is not a client signal. Unknown originators stay unlabeled. Menu bar, mobile rows, and the mobile long-press sheet show the client beside Codex. ChatGPT desktop threads cannot be moved into tmux: their original app process cannot be exited by the native record, so resuming would create two clients. Other native sessions keep their existing actions. | `internal/transcript/codex.go`, `internal/radar/agents.go`, `internal/app/adopt.go`, `macapp/Sources/GtmuxBar/MenuView.swift`, `mobileapp/src/ui/rowSheetModel.ts` |
| Phone terminal | Default wrapping fits the phone. Original/Wrap uses the source tmux columns with horizontal panning, preserving wide Codex TUI frames; older servers fall back to the widest captured row in terminal cells. This does not alter Chat history. | `mobileapp/src/ui/NativeTerm.tsx`, `mobileapp/src/ui/term.ts` |

For diagnosis, `gtmux agents --json` shows pane and role; `gtmux events --json`
and `gtmux logs --component hook --json` show events and suppression reasons.
Check a live HQ's **Codex process arguments** for the Ghostty filter: an old process
keeps its old options after gtmux itself is upgraded. A late real approval menu may
be found by radar's next poll. Phone VoiceOver and small-screen layout still need
physical-device verification.
