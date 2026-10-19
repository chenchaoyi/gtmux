# DeepSeek Harness terminal clients

DeepSeek Harness keeps agent key `dsh`, its label and official whale icon. A
terminal client is an internal adapter choice, not another agent brand. The first
adapter targets dst 0.16.1's native DSH backend and DSH v4 session contracts. Other
dst backends do not become DSH sessions. Web conversations retain the existing
external, read-only behavior and opt-in follow policy.

## Attachment and input

`internal/agentclient` verifies a private, five-second native witness against the
canonical DSH home, exact root session, tmux server epoch, live process ancestry,
dst's foreground injection registry, mounted-session registry and a live v4 root
from the native session registry. A fresh session may precede its first durable
artifact; an existing corrupt or unsupported artifact is still refused. A background
mount alone cannot select a pane. Conflicts and stale evidence revoke input. Radar
and transcripts use the current binding; the same session is suppressed from the
external list. All surfaces consume the shared core and existing official assets.

The bridge is an admitted native component with one declared `commands.invoke`
capability. Its `gtmux-send` command receives `CommandInvocation.agent`; it checks
home, session, process instance and binding before calling the native `followup`
queue with an identified public user message. No second session writer or model
proxy is opened. A 128-bit command token selects a private request of at most 1 MiB;
request text never travels in shell argv. Exclusive claiming precedes enqueue, and
only a matching native receipt proves acceptance. Acceptance does not prove model
execution or completion. An unknown claim fails without automatic re-enqueue.
Completed retry metadata expires after 24h; unresolved claims stop growth at the
4,096-entry spool cap. Expired unclaimed bodies are removed on the next send.

The existing dispatch pipeline protects drafts. The adapter additionally requires
dst's compact composer and insert mode before paste and Enter; pickers, questions,
expanded editors and Vim normal mode are refused. The command token must remain
fully visible to the shared paste verifier; narrow/wrapped composers can refuse.
Phone sends retain owner/share/consent checks and never override a draft. Native
input is not offered for Web histories. CLI `--no-enter` only stages a draft and
does not carry a native acceptance claim.

## Message evidence and review

One asynchronous reconciler scans committed appends with a bounded cursor per
verified foreground session, capped at 16 readers and 100 public messages/4,096 new
source records per tick. Compressed frames remain bounded by the existing DSH
reader. Journal deduplication is an incremental, bounded key index, not a full
lifecycle decode for every request. Only public user and assembled assistant text
is projected; reasoning, injections, tool results and replacement/replay copies
are excluded. Source sequence gaps can therefore be normal.

`ConversationMessage` identifies the source message and provides a short public
summary. `transcript --messages` returns bounded pages of full public text.
Reading and HQ event consumption never declare review: `--review-through` is an
explicit local declaration limited to returned messages. The HQ charter teaches
that distinction. Source failures, replacement and observed truncation produce
`ConversationGap`; replacement resets saved review coverage. Retention limits,
unobserved rewrites and unavailable source material preclude an exactly-once or
complete-history guarantee. Older Web and background intervals are not silently
enrolled by selecting a terminal conversation; explicit history reads can inspect
them. No automatic approval gate is introduced.

## Installation and validation boundaries

The installer writes gtmux-owned native modules, a component manifest and launch
overlays. For an existing dst profile, it validates the YAML sequence and adds an
owned block to `profiles/dsh-tui/cordis.patch.yml`, preserving other text and custom
tags, with a content-addressed backup. Uninstall removes only that block and owned
artifacts. Ambiguous YAML, foreign artifact ownership and linked profile files are
refused. Nonempty flow-style root arrays require manual conversion to a block
sequence. No third-party npm package is installed or upgraded by gtmux.

Ordinary `dst --backend dsh` uses the profile bridge. `spawn --agent dsh` selects
this declared client with its launchpad disabled and an explicit current workspace,
so a new goal waits for both the chat composer and the native receiver. New panes
start the client directly instead of typing into an initializing shell, and the
configured DSH home is explicit; terminal resume records remain specific to the verified home
and client. Web adoption stays disabled. Model selection stays in dst; generic
`spawn --model` and one-shot execution are refused for this adapter.

Doctor separates installed artifacts/profile configuration from a fresh loaded
attachment and its revocable input capability. It never restarts existing work.
Passing author fixtures or component admission is not live TUI acceptance; delivery
reports must separately name screen, native queue and real-model coverage.

Primary interface source: [dst v0.16.1](https://github.com/ccch1mneyyy/dsh-TUI/tree/73ff99fbed56563bf154d719effd09f9f9725349)
(foreground discovery, host admission and commands),
[DSH session contracts](https://github.com/deepseek-ai/deepseek-harness/tree/dsh-v0.2.0-rc.2/packages/core/session).

## Native terminal status during repaint

A verified lifecycle-aware terminal adapter uses a separate native event snapshot,
bound to its current harness, data home, session, pane and client incarnation.
Start, question, resume and end hooks determine the phase; frame/CPU changes and
generic active/finished caches cannot override it. In particular, a finished-time
cache may have been inferred by radar and is not proof of a native completion.

A later explicit turn boundary in the exact DSH durable history can recover a
missed hook. Default history state, unrelated sessions and older boundaries cannot.
The reader uses the existing incremental DSH catalog, not the global event journal.
Without either source the row stays neutral (`running`), with no invented completion
time. Existing sessions need a natural hook or explicit history boundary; there is
no guessed backfill or extra plugin to install. A native question remains waiting
through repaint until a native resume/end or later explicit turn boundary settles it.
The adapter declares this capability; replacing the community client keeps `dsh`
as the harness identity. Other integrations retain their existing documented fallbacks. A tracked goal still
blocked in the input box retains the separate undelivered-dispatch waiting guard.
