# Desktop conversation reader

**English** · [中文](desktop-conversation.zh.md)

Open a verified external Codex conversation to read its recorded prompts, public
replies and tool steps. Public commentary appears during work, before the final answer.
This is a user-initiated read; it does not enable HQ follow, notifications or knowledge
collection. Follow settings remain separate.

## Entry and behavior

- **Mac:** click the external Codex row, or choose Read conversation from its context menu.
  A resizable native window shows read-only history; the settings icon still opens
  follow settings. Polling pauses when the window is hidden, minimized or the app is inactive.
- **Phone:** tap the external Codex row to open read-only Chat. Use its long-press menu for
  follow settings. A guest cannot open the conversation.
- **iPad:** the same reader occupies the workspace detail area, with bounded reading
  width and safe-area padding. Selecting another conversation replaces the reader.
- **CLI:** `gtmux transcript <session_id> --json` reads the same core transcript.
  `--home-id <home_id>` binds the request to the row’s Codex home.
  `--etag <revision>` returns `unchanged:true` with `turns:null` when unchanged.
- **Web:** desktop rows remain status diagnostics in this batch; no desktop reader is wired.

Visible active readers make serial reads about every two seconds. Closing the reader
or backgrounding mobile cancels scheduled reads and ignores late replies. A new Mac, Codex home or
conversation cannot inherit an old response. Transient errors preserve loaded history
and offer Retry. No terminal, composer, approval or Move to tmux is offered: continue
the conversation in the original client.


External Codex observations in the configured home need a matching persisted rollout
before entering the live list: a shared backend PID can also serve transient helper
calls. Early hooks can appear on the next poll after the rollout exists; this display
gate deletes no records. IDs are never merged by title, cwd, PID or session-tree root.
The observed Desktop page-open envelope with exactly one string/null `page_id` is
not user dialogue. Genuine text after it stays visible; malformed envelopes and
literal mentions in prose are preserved. Other-home observations remain outside
this home's rollout evidence.

For Codex TUI creation, `originator=codex-tui` can coexist with rollout
`source=cli` or `source=vscode`. Neither backend source alone identifies the
current client. Process-backed recovery accepts either known source only after
verifying that the callback's exact process is the live foreground terminal
client. A shared daemon callback does not prove which terminal owns the thread;
it remains unbound without a separate exact binding witness.

## Native reading layout

The Mac reader follows mobile Chat's hierarchy and controls while retaining native
light/dark colours. Its compact header shows the harness/source and read-only state;
optional launch evidence is behind an information button. Each turn has one sender
identity and one assistant header, with the existing official icons and user/HQ
avatars. Core `from` attribution distinguishes HQ/delegates from the human reader.
Public Markdown uses the existing native reading renderer and scale. Timestamps use
the same fifteen-minute/day separators as the HQ reader.

Replies can be folded individually or all at once. A folded reply retains its preview
and icon-only copy/share actions for all recorded reply text. Tool groups start folded,
show operation-specific symbols and open a compact list; each recorded call can expose
its original input, working directory and truncation notice. Long input scrolls both
ways and remains selectable/copyable; recorded input does not attest successful execution.

The reader reuses native Markdown, avatar, copy-feedback, timeline and bounded tail
positioning components rather than embedding the React Native app or a second Web
client. Initial entry and the latest-message action reveal the tail; refresh while
reading older messages preserves the reader's position. Loading and retry states
remain explicit, and these controls never grant input or change follow consent.

## Data and boundaries

Only the Go core resolves files. The exact conversation ID must be verified by the
Codex thread metadata. Creation originator does not limit reading; unverified IDs fail
closed. Modern thread IDs take precedence over tree-root session IDs. `$CODEX_HOME` remains supported. The endpoint is owner/paired-device-only:
`GET /api/session/transcript?session_id=<id>&home_id=<home_id>`; guests are rejected before content reads.
It uses the public parser, omitting private analysis and injected instructions, and
combines verified resumed rollout files. The revision is read before content so an
append during parsing triggers another read. Responses retain at most 300 turns and
512 KiB; size omissions are reported. Conditional reads preserve existing history.

Mac consumes the CLI and mobile consumes the HTTP contract; neither reads Codex storage.
A runtime handoff is not implemented. A new CLI resume can create another client of the
same conversation, and desktop-owned tools may still need their original client.

The proposed [Codex client lifecycle design](codex-client-lifecycle.md) separates
creation source, current control and persisted history. It is a research/implementation
plan, not an available transfer action.

## Verification

Core tests cover identity refusal, guest/owner scope, public intermediate content,
continuations, revisions, bounds and no permission changes. Mobile tests cover the shared
Detail entry, serial polling, background/unmount cancellation, server/conversation
changes, error retention and no input calls. Mac tests cover CLI arguments, serial
reads, late results, inactive state and bilingual light/dark layout fixtures.
Physical phone/iPad layout and VoiceOver acceptance remain pending; component and
synthetic window tests do not establish device acceptance.

## Codex history retention

The shared Codex reader scans the rollout once and retains up to 800 parsed
turns, excluding tool results, compaction snapshots and other non-chat records.
Large tool output therefore cannot displace earlier questions just by exceeding
a raw log byte window. Later polls read newly appended complete JSONL records;
a partially written record waits for completion. Cached responses already in use
remain unchanged. Log replacement, shrink and same-size rewrite rebuild the cache.

This applies to tmux and Desktop Codex Chat. HTTP responses still use the
300-turn / 512 KiB budget (keeping the newest turn even if it alone exceeds the
byte budget); the UI initially mounts a smaller window. This is not unlimited
history paging. Collapse all keeps one reply preview per loaded user turn; it
does not delete history or combine separate turns. Terminal scrollback is separate
and is unaffected by this reader change. Updating the Mac server enables this fix;
an app binary update alone does not.
