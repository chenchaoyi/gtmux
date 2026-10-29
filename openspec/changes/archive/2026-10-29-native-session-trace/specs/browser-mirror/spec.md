
## MODIFIED Requirements

### Requirement: Chat (对话) mode mirrors the transcript

The pane view SHALL offer Chat / Terminal presentation modes and a separate Trace session tool; the chat mode SHALL
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
