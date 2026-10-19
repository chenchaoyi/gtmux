---
title: DeepSeek Harness conversations
description: Read Web histories and manage verified DeepSeek Harness terminal sessions.
order: 18
---

DeepSeek Harness keeps one identity and its official whale icon, whether you use
Web or a supported community terminal client. dst is the first terminal adapter;
it is not another agent category. See the [all-agent integration map](agent-integrations.md).

## Web histories

Gtmux discovers saved root conversations under `$DSH_HOME/sessions` (default
`~/.dsh/sessions`). External conversations are read-only on the menu bar, phone,
iPad and Web. Explicit CLI reads need no plugin, credentials, model request or
session writer lease:

```sh
gtmux transcript <session_id> --json
```

Discovery supports DSH 0.2.0-rc.2 v4 JSONL and concatenated checksummed Zstandard
frames. Partial appends wait; unknown generations, ambiguous IDs and corrupt
sources are refused. Public text and tool steps are readable; private reasoning,
raw tool results, injected instructions and replacement/replay copies are excluded.
Detection alone does not enable HQ follow, notifications or learning. Use the
existing `gtmux follow <session_id> …` controls for one external conversation.
Web histories cannot receive input, be adopted, or be relaunched by gtmux.

## Native bridge and terminal setup

No separate gtmux npm package is needed. Gtmux bundles bridge scripts; DSH supplies
its own Cordis runtime. Install the official DSH CLI and the community dst client
separately and complete dst's first profile initialization, then run:

```sh
gtmux install hooks --agent dsh
# --agent auto and the regular installer also detect dsh/dst.
dst --backend dsh
gtmux doctor
```

The installer writes its owned modules, native component manifest and overlays
under `$DSH_HOME/gtmux`. When a dst profile already exists, it adds a marked block
to `profiles/dsh-tui/cordis.patch.yml`, preserving other text and custom tags and
saving a content-addressed backup. Uninstall removes only that block and gtmux's
owned files. It refuses linked/ambiguous configuration and foreign artifacts.
Nonempty flow-style root arrays require conversion to a block sequence. No
third-party client is installed or upgraded automatically, and no process is restarted.

For native **Web lifecycle events**, deliberately select the separately printed
`dsh web --patch '…/gtmux/gtmux.cordis.yml'` on your next Web launch. Reading saved
history does not need it. The bridge uses DSH's own `session/event` and
`user-questions/request`; it does not use Codex compatibility hooks, answer questions
or approve permissions. Doctor distinguishes installation from live terminal
attachment and input permission; it cannot prove a running Web loaded the overlay.

## Managed terminal conversations

The terminal adapter targets dst 0.16.1's DSH backend. A fresh native bridge must
agree with the exact DSH home/session, process ancestry, tmux server and current
foreground selection before the core presents a managed DeepSeek Harness pane.
Background mounts, another dst backend, stale records and ambiguous bindings do
not authorize input. The same conversation is suppressed from external history
rows while managed. Mac, phone, iPad, Web and CLI use the same core data and icon.

Use ordinary pane capture/focus/chat/send workflows. `gtmux spawn --agent dsh`
launches the declared client directly in its chat workspace, skipping the landing
screen; a native live root can be recognized before its first history file. Verified terminal resume records relaunch that same
home/client. Select the model in dst: this adapter refuses generic `--model` and
one-shot spawn. Submitted text travels through a session-targeted native command,
with a private one-use request and an acceptance receipt from the actual receiving
agent. It protects existing drafts and refuses a switch, menu, question, Vim normal
mode or expanded editor. Narrow wrapped command tokens may also refuse. An
acceptance receipt means queued, not executed or completed. Unknown delivery
outcomes are not automatically re-enqueued. Completed retry metadata lasts 24h;
unresolved claims eventually stop new sends at the bounded spool cap.

## HQ public-message review

The reconciler incrementally recovers committed public user and assistant messages
for verified foreground intervals; it does not enroll older Web/background activity.
`ConversationMessage` events carry stable identity and a short public summary.
For full public text, use bounded pages:

```sh
gtmux transcript <session_id> --messages --after-seq -1 --limit 100 --json
# After judging the returned messages, explicitly declare review of that page:
gtmux transcript <session_id> --messages --after-seq -1 --limit 100 --review-through <last_message_seq> --json
```

Continue from `reviewed_through`; `has_more` means another page remains. Reading
messages, consuming the HQ event stream, notifications and input receipts never
mark a message reviewed. Review is observation, not a pre-execution approval gate.
Source failures/replacement/truncation produce `ConversationGap`; replacement resets
review coverage. Reconcile missing history rather than claiming complete coverage.
Only text messages are in this page view; tool/permission interactions remain in
the original client. Usage metering is not implemented for DSH.

## Sources and validation

The [official favicon](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg)
is the shared icon source. Native APIs:
[DSH sessions](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/core/session),
[dst v0.16.1](https://github.com/ccch1mneyyy/dsh-TUI/tree/73ff99fbed56563bf154d719effd09f9f9725349).
See the [terminal design](../design/dsh-terminal-clients.md) for trust, retention
and validation boundaries. Author fixtures, actual component admission, live TUI
queue acceptance and model execution are separate evidence; consult the delivery
report for which were established on the installed version combination.
