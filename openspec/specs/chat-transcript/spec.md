# Chat Transcript Specification

## Purpose

Turn each coding agent's on-disk session log into a clean chat history — one user
prompt, the agent's reply, and the tool calls it ran along the way — so the phone
and browser can show a readable "对话/chat" view of what an agent did, not just
its raw terminal screen. The parser is agent-agnostic over a shared Turn model and
is served read-only via `GET /api/transcript`.

## Requirements

### Requirement: Parse an agent session log into ordered turns

The system SHALL parse a pane's resumable agent session log into an ordered list of
`Turn`s, each a user `Prompt` plus the agent's reply, agent-agnostically over a
shared model. It SHALL support Claude Code logs (`~/.claude/projects/<slug>/<id>.jsonl`)
and Codex rollout logs (`$CODEX_HOME/sessions/.../rollout-*-<id>.jsonl`), keyed to
the pane's session, and return an empty list when the pane has no resumable session
or no readable log.

#### Scenario: Claude log becomes turns

- **WHEN** a pane has a Claude session log with user prompts and assistant replies
- **THEN** the parser returns one `Turn` per user prompt, each carrying the reply
  that followed it

#### Scenario: Codex log becomes turns

- **WHEN** a pane has a Codex rollout log (`user_message` / `function_call` /
  `agent_message` / `task_complete` records)
- **THEN** the parser returns turns with the same `Turn` shape as Claude

#### Scenario: No session

- **WHEN** the pane has no resolvable session log
- **THEN** the parser returns an empty list (not an error)

### Requirement: Keep every reply segment with its interleaved steps

A turn's reply SHALL be modelled as ordered `Segments`, where each segment is one
assistant text bubble plus the tool steps that ran AFTER it (text → tools → text →
…), preserving chronological interleaving. The system SHALL keep EVERY assistant
text segment of a turn (not only the last), and attach each tool call as a `Step`
(`kind`, tool `title`, short `detail`) to the segment it followed. A `Response`
field SHALL also be derived (all segment texts joined by a blank line) for
back-compat / simple consumers.

#### Scenario: Multi-segment reply with steps between

- **WHEN** an assistant emits text, then runs tools, then emits more text in one turn
- **THEN** the turn has multiple segments in order, each text segment carrying the
  steps that ran after it, and `Response` is all the texts joined

#### Scenario: Leading tools

- **WHEN** a turn's first activity is tool calls before any assistant text
- **THEN** those steps attach to a leading segment so no step is dropped

### Requirement: Recover rejection feedback as turns

The system SHALL surface a user's tool-rejection feedback (the note typed when
declining a tool call) as part of the transcript, while NOT treating a bare
rejection with no feedback as a turn of its own.

#### Scenario: Rejection with a note

- **WHEN** the user rejects a tool call and types a reason
- **THEN** that feedback appears in the transcript

#### Scenario: Bare rejection

- **WHEN** the user rejects a tool call with no note
- **THEN** no empty turn is produced

### Requirement: Strip harness/meta noise

The system SHALL omit harness-injected and meta content (e.g. system reminders,
command wrappers, meta-prompts the agent harness inserts) from prompts and replies
so the chat shows only human-meaningful conversation.

#### Scenario: Meta prompt hidden

- **WHEN** a log line is a harness/meta prompt rather than a real user instruction
- **THEN** it is not shown as a user turn

### Requirement: Incremental tail cache

The system SHALL cache parsed turns per session and, on refetch, resume from the
last byte offset rather than re-reading the whole log — extending an open turn in
place, splicing on new turns, and never duplicating or dropping turns. It SHALL cap
how much tail it reads and how many turns it retains.

#### Scenario: Log grows

- **WHEN** the session log gains content since the last parse
- **THEN** the cache resumes from the saved offset, updates the open turn and
  appends new turns, without duplicating earlier turns
