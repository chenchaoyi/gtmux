## MODIFIED Requirements

### Requirement: Speaker identity sheets

User, sender and agent avatars in conversations SHALL be keyboard/touch-activatable
and open an adaptive identity sheet using the app's avatar and information hierarchy.
Sheets SHALL distinguish users, HQ and agents, show available role/session/status/source
information and offer navigation only to a currently authorized matching session.
Owner agent sheets SHALL read recorded facts from the existing participant-profile
endpoint, with known model/version/provider/permission fields and source/time disclosure.
Historical message profiles SHALL use recorded session and time, including older sessions
of the same agent provider. Missing message identity and guest connections SHALL NOT
request current facts as a substitute. Closing or changing credentials SHALL retire
pending reads. Missing personal/device information and historical identities SHALL NOT
be replaced with inferred current data. Settings SHALL use an icon with a localized
accessible name.

#### Scenario: Inspect a historical speaker

- **WHEN** an older message belongs to the same provider as the currently working session
- **THEN** the sheet requests facts for the message's recorded session and time
- **AND** it does not assign the current session's status or navigation to that message

#### Scenario: Retire profile reads

- **WHEN** a profile closes, another profile opens or access is revoked
- **THEN** a delayed response cannot repaint a different profile or remain visible after revocation

#### Scenario: Missing identity or guest access

- **WHEN** a message lacks recorded identity or the reader is a guest
- **THEN** the sheet explains the unavailable recorded facts without querying a current profile

### Requirement: Transcript trace

The browser SHALL provide an ordered, searchable index of public prompts, assistant
segments and recorded tool input, falling back to recorded summaries when input is
absent. It SHALL preserve input whitespace and show recorded working directory and
truncation. Agent, loaded turn and session boundaries SHALL remain distinguishable,
including mixed-agent HQ history. Search SHALL cover all loaded records, including
recorded input, directory, agent and session; type and history-session filters SHALL
compose. It SHALL NOT invent request timing, token usage, private reasoning, tool
output or tool completion. Session status SHALL come from roster evidence and a
historical-session filter SHALL NOT imply that the historical session is working.

#### Scenario: Inspect a call

- **WHEN** the reader selects a tool event in the trace
- **THEN** its recorded input, or its summary if no input was recorded, is readable and copyable without altering whitespace
- **AND** a recorded directory or truncation flag is visible without claiming the complete raw request is available

#### Scenario: Inspect mixed-agent conversation history

- **WHEN** the transcript contains several sessions or agent providers
- **THEN** the list identifies each record's agent and loaded turn, exposes session boundaries and allows a history-session filter
- **AND** no prompt timestamp is presented as the timestamp of an individual tool invocation

#### Scenario: Read a long or incomplete transcript

- **WHEN** thousands of records, missing timestamps, duplicate prompts, tools-only turns or legacy responses arrive
- **THEN** the index preserves recorded ordering, keeps distinct entries and renders at most 200 record rows per page with explicit pagination
- **AND** search and the bounded overview cover the complete loaded index, including its final record
- **AND** empty data and empty filter results have distinct localized states

#### Scenario: Navigate the overview without tiny targets

- **WHEN** a reader locates an event from the overview
- **THEN** a native range control supports touch and keyboard positioning and decorative bars do not become tiny focusable buttons

### Requirement: Closeable trace details

The trace inspector SHALL be closeable. Closing it SHALL give the event list the full
available width and allow longer previews. Closing SHALL survive polling and filtering;
selecting an event or choosing Show details SHALL reopen it. Width-appropriate compact
views SHALL drill into details and return to the same selected row and focus, rather
than dividing an already narrow reading area. Identifiable records SHALL retain their
selection across history additions. Selected text and focused detail controls SHALL
hold their displayed revision during updates and resume after release. Inspecting or
manually scrolling SHALL stop automatic following. Viewer-owned observers and timers
SHALL be disposed when the trace is retired.

#### Scenario: Read with the inspector closed

- **WHEN** the user closes event details and a transcript update arrives
- **THEN** the event list remains full width and the inspector remains closed

#### Scenario: Read a call on a phone or compact tile

- **WHEN** a reader selects a record in a narrow content pane
- **THEN** details use the reading area and the underlying list is not an interactive accessibility target
- **AND** closing details restores the list and focuses the original row

#### Scenario: Preserve reading while history changes

- **WHEN** older or newer identifiable history is inserted while a record is selected
- **THEN** the same record remains selected, without substituting another record at its previous array index
- **AND** a selected text range remains intact until the reader releases it

### Requirement: Corresponding conversation controls use consistent semantics

The browser focus and tile views SHALL order Chat, Trace, then Terminal for pane
agents. Chat and Terminal SHALL retain their relative order with the native App; this
requirement SHALL NOT imply that native App Trace has been implemented. Readable
external sessions SHALL offer read-only Chat and Trace and SHALL NOT offer Terminal
or a composer. Unreadable native sessions SHALL remain status-only. Plain panes SHALL
NOT offer Chat or Trace and opening them SHALL NOT replace the saved agent mode.
The initial pane-agent default SHALL remain Terminal. Expanding a tile or moving to
an adjacent agent SHALL preserve the current supported mode. Plain-pane titles SHALL
remain consistent between roster and focus. Workbench-only Diff SHALL remain available.
Visible status and accessible status names SHALL use the same mapping. All-panes fold
controls SHALL use Collapse all and Expand all in English and 折叠全部 and 展开全部 in Chinese.

#### Scenario: Switch from a conversation to a shell

- **GIVEN** an agent uses the saved Chat mode
- **WHEN** the user opens a plain shell pane and then an agent
- **THEN** the shell exposes only Terminal and the agent restores Chat

#### Scenario: Corresponding agent tabs

- **WHEN** the user opens an agent in focus or a workbench tile
- **THEN** Chat appears before Terminal in either language

#### Scenario: Read an external conversation trace

- **WHEN** an external session is declared readable by the server
- **THEN** its entry identifies the read-only conversation and offers Chat and Trace through its exact session/home scope
- **AND** changing session, home or credential retires previous reads before they can repaint the new reader
- **AND** reading, filtering and copying never sends input to a real pane

#### Scenario: Expand a trace or move to the next pane

- **WHEN** Trace is selected and the reader expands a tile or moves to the next agent
- **THEN** the new focus remains in Trace instead of silently switching to Terminal
