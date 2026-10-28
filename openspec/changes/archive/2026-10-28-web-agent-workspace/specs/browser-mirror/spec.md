## ADDED Requirements

### Requirement: Browser conversation workspace
The browser SHALL present prompts, chronological assistant segments, and expandable tool summaries consistently in focused and workbench chat. Replies SHALL show the turn's agent identity, supervisor sessions SHALL carry the HQ role mark, and text SHALL be copyable on pointer and touch devices. Conversation input SHALL use the existing scope-gated send path and remain intact during transcript refreshes.

#### Scenario: Continue a conversation
- **WHEN** an owner opens a conversation and sends a follow-up
- **THEN** the browser submits to that pane, preserves an unsent draft during polling, and retains readable spacing between messages

### Requirement: Transcript trace
The browser SHALL provide a searchable ordered trace of recorded prompts, assistant segments and tool summaries, with selectable details and copy. It SHALL identify the scope of recorded data without inventing request timing, token usage or tool results.

#### Scenario: Inspect a call
- **WHEN** the reader selects a tool event in the trace
- **THEN** its recorded title and argument summary are readable and copyable

### Requirement: Local browser entry
The menu bar SHALL offer a local browser workspace at 127.0.0.1, authenticated through a short-lived one-time code. If no serve is available it SHALL start only a loopback listener. It SHALL not enable a tunnel or rewrite an existing remote service.

#### Scenario: Open without remote access
- **WHEN** the user opens the local workspace with remote access off
- **THEN** a loopback-only serve starts and the browser redeems a one-time code for its own authenticated credential

## MODIFIED Requirements

### Requirement: Chat (对话) mode mirrors the transcript

The pane view SHALL offer a 对话/轨迹/终端 (chat/trace/terminal) switch; the chat mode SHALL
render the pane's parsed transcript (see `chat-transcript`) by polling
`GET /api/transcript` — a user-prompt bubble followed by the reply's `segments` as
separate speech bubbles with the interleaved tool steps as collapsible groups,
mirroring the phone's chat view. Chat SHALL use the same authorized free-text composer as Terminal. Its
waiting card MAY send a parsed option's number when the caller is authorized, as
defined by the input-capability requirement below.

#### Scenario: Switch to chat mode

- **WHEN** a session is selected and the user switches to 对话/chat mode
- **THEN** the browser renders the parsed transcript as a conversation (prompt
  bubble, segmented reply bubbles, collapsible steps) and keeps it fresh by polling
  `/api/transcript`

#### Scenario: Free-text entry in Chat

- **WHEN** the chat mode is displayed
- **THEN** an authorized caller can type and send a message without leaving Chat; a read-only caller sees an explanation instead, while authorized structured-choice replies remain available

### Requirement: Corresponding conversation controls use consistent semantics

The browser focus and tile views SHALL order Chat, Trace, then Terminal for agents, matching the phone and iPad. Plain panes SHALL NOT offer Chat or Trace and opening them SHALL NOT replace the saved agent mode. The initial default SHALL remain Terminal. Plain-pane titles SHALL remain consistent between the roster and focus. Workbench-only Diff SHALL remain available. Visible status and accessible status names SHALL use the same mapping. All-panes fold controls SHALL use Collapse all and Expand all in English and 折叠全部 and 展开全部 in Chinese.

#### Scenario: Switch from a conversation to a shell

- **GIVEN** an agent uses the saved Chat mode
- **WHEN** the user opens a plain shell pane and then an agent
- **THEN** the shell exposes only Terminal and the agent restores Chat

#### Scenario: Corresponding agent tabs

- **WHEN** the user opens an agent in focus or a workbench tile
- **THEN** Chat appears before Terminal in either language
