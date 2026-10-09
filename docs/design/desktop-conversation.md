# Desktop conversation reader

**English** · [中文](desktop-conversation.zh.md)

Open a detected ChatGPT desktop Work conversation to read its recorded prompts, public
replies and tool steps. Public commentary appears during work, before the final answer.
This is a user-initiated read; it does not enable HQ follow, notifications or knowledge
collection. Follow settings remain separate.

## Entry and behavior

- **Mac:** click the desktop row, or choose Read conversation from its context menu.
  A resizable native window shows read-only history; the settings icon still opens
  follow settings. Polling pauses when the window is hidden, minimized or the app is inactive.
- **Phone:** tap the desktop row to open read-only Chat. Use its long-press menu for
  follow settings. A guest cannot open the conversation.
- **iPad:** the same reader occupies the workspace detail area, with bounded reading
  width and safe-area padding. Selecting another conversation replaces the reader.
- **CLI:** `gtmux transcript <session_id> --json` reads the same core transcript.
  `--etag <revision>` returns `unchanged:true` with `turns:null` when unchanged.
- **Web:** desktop rows remain status diagnostics in this batch; no desktop reader is wired.

Visible active readers make serial reads about every two seconds. Closing the reader
or backgrounding mobile cancels scheduled reads and ignores late replies. A new Mac or
conversation cannot inherit an old response. Transient errors preserve loaded history
and offer Retry. No terminal, composer, approval or Move to tmux is offered: continue
the conversation in ChatGPT desktop.

## Data and boundaries

Only the Go core resolves files. The exact conversation ID must be verified by the
existing Codex `session_meta.originator` mapping; unknown and terminal identities fail
closed. `$CODEX_HOME` remains supported. The endpoint is owner/paired-device-only:
`GET /api/session/transcript?session_id=<id>`; guests are rejected before content reads.
It uses the public parser, omitting private analysis and injected instructions, and
combines verified resumed rollout files. The revision is read before content so an
append during parsing triggers another read. Responses retain at most 300 turns and
512 KiB; size omissions are reported. Conditional reads preserve existing history.

Mac consumes the CLI and mobile consumes the HTTP contract; neither reads Codex storage.
A runtime handoff is not implemented. A new CLI resume can create another client of the
same conversation, and desktop-owned tools may still need their original client.

## Verification

Core tests cover identity refusal, guest/owner scope, public intermediate content,
continuations, revisions, bounds and no permission changes. Mobile tests cover the shared
Detail entry, serial polling, background/unmount cancellation, server/conversation
changes, error retention and no input calls. Mac tests cover CLI arguments, serial
reads, late results, inactive state and bilingual light/dark layout fixtures.
Physical phone/iPad layout and VoiceOver acceptance remain pending; component and
synthetic window tests do not establish device acceptance.
