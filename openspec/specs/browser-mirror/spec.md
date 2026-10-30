# Browser Mirror Specification

## Purpose

Let a person on any computer watch a Mac's tmux agent sessions in a plain web
browser, with zero install — a mirror of the agent radar and live panes with
input only where the owner or a share link grants it,
served by `gtmux serve` / `gtmux tunnel` and reachable over LAN or the hosted
tunnel when the network permits it. New owner pairing links use a one-time code
from the Mac; a guest can use a share link. The browser also accepts legacy token
fragments for compatibility. A phone-to-computer handoff action was removed in #512;
the archived browser-mirror proposal records the earlier design, not the current UI.

## Requirements

### Requirement: Browser access copy serves owners and guests

The browser access page SHALL explain that its code field accepts either an
owner pairing code or a guest share code. A rejected code SHALL prompt the
reader to check it or get a new one without assuming a share-link sender.
An empty chat view SHALL point to Terminal for the current screen without
misstating how the agent transcript is recorded.

#### Scenario: A Mac owner mistypes a pairing code

- **WHEN** enrollment refuses the code entered on the browser access page
- **THEN** the page gives a usable next step without telling the owner to ask
  another person to resend a share link

### Requirement: Browser pairing via a one-time enroll code

The web UI SHALL authenticate by redeeming a short-lived, single-use enroll code
into a per-device token; a newly generated owner pairing link SHALL carry a code,
not the master token. The browser SHALL read `/#c=<code>` for owner pairing and
`/#code=<code>` for share codes, redeem through `POST /api/enroll`, and store the
resulting token in localStorage. It SHALL remove the credential fragment before
starting redemption. Legacy `#g=` / `#t=` token fragments remain accepted and are
also removed from the address bar; this compatibility path is not code redemption.

#### Scenario: Pairing link authenticates the browser

- **WHEN** a browser opens `/#c=<valid-code>`
- **THEN** the page redeems the code via `/api/enroll`, stores the returned device
  token, removes the `#c=` fragment, and shows the radar

#### Scenario: Expired or invalid code

- **WHEN** a browser with no saved credential opens `/#c=<expired-or-unknown-code>`
- **THEN** the page explains that access is not set up, offers a code field and
  the Mac’s `gtmux pair` path, and does not load agents

### Requirement: Pairing code from the Mac

A browser owner SHALL be able to obtain a one-time pairing link with `gtmux pair`
on the Mac. Interactive `gtmux serve` and `gtmux tunnel` banners SHALL advertise
browser addresses, and print a code link when a code is available. A loaded
always-on tunnel is reused; if its URL file is missing, the command points to
`gtmux tunnel --status` instead of inventing an address. If tunnel code minting is
unavailable, its legacy QR may contain the master token, but its printed bare
browser URL does not by itself authenticate a new browser.

The phone's former “Open on computer” settings row was deliberately removed by
commit `61bf41e85669e9d5dcdf0c9fc46862eebeade853` (#512). An API client helper or an
archived checked task is not evidence that the current phone offers that action.

The `gtmux serve` banner SHALL print the token and the pairing link only when it prints
to a terminal. Written anywhere else, which in practice is the LaunchAgent's capture
(`logs/serve.stderr`), it SHALL print neither, SHALL NOT mint a code for it, and SHALL say where
the token lives and how to pair instead; serve SHALL also narrow its stdout and stderr
to their owner (0600) when they are plain files. Either credential gets a device full
control of the Mac, and that log is created world-readable under directories other
accounts can list: before this it held the token once per restart and a fresh pairing
code with each.

#### Scenario: Serve banner advertises the browser (LAN)

- **WHEN** `gtmux serve` starts in a terminal
- **THEN** its banner prints the reachable LAN browser URL(s), the token and a one-time
  pairing link

#### Scenario: Serve under launchd writes no credential to its log

- **WHEN** `gtmux serve` starts with its output going to a file
- **THEN** the file is made readable by its owner only, and the banner in it names
  neither the token nor a pairing link, pointing at `gtmux pair` instead

#### Scenario: Tunnel banner advertises the public browser address

- **WHEN** the selected tunnel starts and a pairing code is minted
- **THEN** its banner prints that tunnel’s public HTTPS browser URL (for hosted
  Standard, `https://gtmux-<id>.ccy.dev/`) and a one-time pairing link
- **AND** access still requires a working Mac-to-tunnel and browser-to-public-endpoint route

#### Scenario: Pair a new browser from the Mac

- **WHEN** the owner runs `gtmux pair` against a reachable local serve
- **THEN** it prints a one-time browser pairing link; opening it redeems the code
  for that browser’s own credential

### Requirement: Live pane mirror that fits the browser window

Selecting a session SHALL render that pane's live screen with xterm.js, updated by
polling `GET /api/pane`, and SHALL refit the terminal to the browser window on
resize. Resizing SHALL change only the VIEW; it SHALL NOT resize the source Mac's
tmux pane.

#### Scenario: Live mirror updates

- **WHEN** a session is selected and its pane content changes on the source Mac
- **THEN** the browser mirror reflects the new content within the poll interval

#### Scenario: Window resize refits the view only

- **WHEN** the browser window is resized
- **THEN** the terminal refits to the new size and the source Mac's pane width is
  unchanged

#### Scenario: A phone-width window keeps the pane's controls on screen

- **WHEN** a pane is open in a window narrower than 800px
- **THEN** its top bar wraps instead of overflowing: back, title and server stay on the
  first row, the title truncating first, and the identity and input chips and the
  controls follow on the rows below; nothing in the bar lies outside the window and the
  page does not scroll sideways
- **AND** the appearance panel opens below the bar, not over it

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

### Requirement: Reachable over LAN and tunnel

The web UI SHALL be reachable on the LAN (`http://<ip>:<port>/`) and remotely via
the existing `gtmux tunnel` public HTTPS hostname (`https://gtmux-<id>.ccy.dev/`),
with the same `/api/*` contract. LAN access requires a reachable bind address and
firewall route; remote access requires both sides to reach the tunnel. Direct may
serve under a `/p<port>/` path, which the browser SHALL preserve for API requests.

#### Scenario: Opened over the tunnel

- **WHEN** `gtmux tunnel` is running and a browser opens the public HTTPS URL with a
  valid pairing link
- **THEN** the web UI pairs and mirrors panes over the tunnel

### Requirement: Desktop "workbench" mode

On a wide screen the served UI SHALL offer a "workbench" mode: a left session/agent
rail plus a freeform board of draggable, resizable pane tiles (multiple panes visible
at once), with layout presets, snap-to-grid and a ⌘K command palette. Polling SHALL NOT open new panels automatically, including when an old saved preference enabled it. Layout changes affect
only this browser; input is available in panes authorized by `/api/share`. It uses
the same authenticated `/api/pane` + `/api/transcript` data as the single-pane mirror.

#### Scenario: Arrange multiple panes

- **WHEN** the user is on a wide screen and adds panes to the workbench board
- **THEN** each pane renders live and can be dragged/resized/arranged, with the
  arrangement applied via presets or manual placement; authorized panes may accept input

### Requirement: Radar parity with the native surfaces

The browser radar SHALL reflect the same status vocabulary as the CLI/menu-bar: it
SHALL show sensed non-tmux sessions as a `source:"native"` "Elsewhere" category and
SHALL render the background-running idle modifier on an idle row whose settled turn
left in-flight background work.

#### Scenario: Native + background-running shown

- **WHEN** `/api/agents` includes a `source:"native"` row and/or an idle row carrying
  the background-running marker
- **THEN** the browser radar surfaces the "Elsewhere" category and the background
  modifier, matching the menu-bar/mobile presentation

### Requirement: Web UI served by `gtmux serve` — view, plus consented per-pane input

`gtmux serve` SHALL serve a self-contained web UI at `GET /`, embedded in the binary
(`//go:embed`, no build step, offline-safe, cgo-free) and served same-origin as the
existing `/api/*`. The UI SHALL present the agent radar and a pane mirror, using the
shared status language (color+shape+glyph; sections waiting→errored→working→idle→running,
where errored groups an idle turn that ended in failure)
identical to the other surfaces. The UI is READ-ONLY BY DEFAULT. It MAY additionally
expose a terminal-input affordance, but ONLY for panes the caller is authorized to type
into, which it SHALL learn from `GET /api/share` (`{input, panes}` for the caller) and
NEVER assume: no input control is shown for a pane the server does not authorize, and
any typing goes through `POST /api/send`, whose server-side gate is authoritative. A
guest whose host has not consented, or a disallowed pane, shows no input control.

The input affordance SHALL communicate capability: authorized panes show the composer without a redundant writable badge. Read-only panes show a read-only label and explain why input is unavailable. The top bar SHALL name the caller's identity
(owner = 全权, guest = 协作视图), resolved from `GET /api/share` (`all:true` ⇒
owner). On a typable waiting pane the parsed numbered options SHALL be live —
one click sends the bare digit (no Enter) via `POST /api/send`, matching the phone's
ApprovalCard; on a view-only pane they stay inert with the reply-elsewhere hint. Only
options `GET /api/options` actually returned SHALL be shown as numbered options: with
none (an open question, or the options request failed) the page SHALL draw no number
buttons, and SHALL say there are no numbered choices and where to answer instead.

#### Scenario: Browser loads the web UI

- **WHEN** a browser requests `GET /` from a running `gtmux serve`
- **THEN** the embedded web UI is returned and renders the agent radar after pairing

#### Scenario: Input shown only for authorized panes

- **WHEN** the UI has fetched `GET /api/share` for the caller
- **THEN** an input affordance is shown ONLY for the panes it lists; other panes stay
  view-only, and a guest with input off sees no input control anywhere

#### Scenario: The server gate, not the UI, is authoritative

- **WHEN** a guest `POST`s `/api/send` for a pane not in its authorized set
- **THEN** the send is refused server-side regardless of the UI state

#### Scenario: A send that does not complete keeps the text

- **WHEN** the composer's send is refused (any non-2xx other than 401, a 403 included) or gets no answer (the request fails)
- **THEN** the text goes back into the box if the box is still empty, a note says why (refused, or not confirmed: it may or may not have reached the Mac), and nothing is sent again by itself

#### Scenario: Capability is stated, not implied

- **WHEN** a caller focuses a pane (or has it on the workbench board)
- **THEN** writable panes show their authorized composer without a writable label, while read-only panes show a read-only label and an input-not-allowed explanation

#### Scenario: Live parsed choices on a typable waiting pane

- **WHEN** a waiting pane the caller may type into shows its structured options
- **THEN** clicking an option sends that digit (no Enter) through `POST /api/send`,
  while a view-only caller sees the same options inert with the reply-elsewhere hint

#### Scenario: A waiting pane with no numbered choices gets no number buttons

- **WHEN** a pane is waiting and `GET /api/options` returns no options, or fails
- **THEN** the reply bar and the chat card show no numbered buttons, say there are no numbered choices and where to answer, and nothing can send a digit; the chat card's heading reads "waiting for your answer", not "approval"

### Requirement: A page that cannot show anything explains itself

When the browser cannot show sessions — it is not paired, or its link expired — the page
SHALL state WHICH of those it is, and SHALL give the exact step that resolves it. It SHALL
name only affordances that exist: an instruction referring to a control the product does
not have is worse than no instruction, because the reader spends their time looking for
it. It SHALL identify itself as gtmux, since a reader may arrive from a bookmark with no
other context. Commands SHALL be rendered as commands rather than as their markup source.
Like every other surface, it SHALL be bilingual.

#### Scenario: An unpaired browser

- **WHEN** the page is opened with no credential
- **THEN** it says the browser isn't paired, and gives the command that produces a link
  plus which of that command's outputs to open

#### Scenario: An expired link

- **WHEN** the page's credential is rejected as expired
- **THEN** it says the link expired — distinct from never having been paired — and how to
  mint a fresh one

#### Scenario: A guest arrives

- **WHEN** an unpaired reader holds a link someone shared with them
- **THEN** the page tells them such a link works without pairing

### Requirement: The pane view shows Codex's pinned prompt in full

In the single-pane terminal view, when a Codex pane's capture starts with Codex's pinned prompt
row — "› ", the prompt with its newlines joined, cut at the pane's width and ended with "…",
ending within two cells of the pane's `cols` — and a later row begins with "› " (the
composer), and exactly one of the conversation's ten most recent logged prompts is longer than
the row's text and begins with it (ignoring whitespace), and the row below does not carry on
with that same prompt, the browser mirror SHALL show that full prompt in a bar above the
terminal and SHALL write the capture to the terminal without the row. The bar SHALL show two
lines at rest, open to the whole prompt on a click, and offer Copy. Cell widths SHALL be the
phone's tmux-measured widths. In any other case — another agent, no `cols`, a row short of the
edge, no match, two matching prompts, no composer row, chat mode, a workbench tile — the capture
SHALL be written exactly as received and no bar shown. While the row is on screen and no prompt
explains it, the view SHALL fetch the conversation log at most once every 4 seconds, and stop
once it is explained.

#### Scenario: A cut Codex prompt in the browser

- **WHEN** a Codex pane's top row is cut at its right edge and the latest logged prompt begins
  with it
- **THEN** the bar shows the whole prompt and the terminal starts at the row below it

#### Scenario: Another agent's identical screen

- **WHEN** a Claude Code pane shows the same bytes
- **THEN** the terminal shows them unchanged and no bar appears

#### Scenario: The prompt is logged late

- **WHEN** Codex moves to a new prompt that its log does not hold yet
- **THEN** the cut row shows as captured, and the bar returns within one fetch of the prompt
  being logged

#### Scenario: The same rules as the phone

- **WHEN** the shared case fixture runs against the browser's matcher and the phone's
- **THEN** both give the same answer for every case

### Requirement: Browser entry preserves its Mac path prefix

The browser mirror SHALL load its scripts, styles, fonts and API calls inside the Mac path prefix even when a share URL has no trailing slash. Its asset base SHALL be set before any external asset reference, remain on the current origin, and exclude query strings and credential fragments. Root and explicit index.html entries SHALL use the same API prefix as the document base.

#### Scenario: A guest opens an existing Direct share URL

- **WHEN** a browser opens `/p35047#code=<valid-share-code>`
- **THEN** the page loads its assets from `/p35047/`, removes the fragment before redemption, and redeems through `/p35047/api/enroll`
- **AND** existing scope checks still determine which sessions and inputs the guest can access

#### Scenario: Root or explicit index entry

- **WHEN** the page opens at `/`, `/index.html`, `/p35047/` or `/p35047/index.html`
- **THEN** its asset URLs and API URLs share the correct root or `/p35047` prefix
- **AND** no credential fragment is added to any asset request

### Requirement: Corresponding conversation controls use consistent semantics

The browser focus and tile views SHALL order Chat then Terminal as presentation
modes for pane agents, matching the native App. Trace and Diff SHALL be available
through a separate session-tools entry. Readable external sessions SHALL offer
read-only Chat with Trace as a tool, without Terminal, Diff or a composer. Unreadable native sessions SHALL remain status-only. Plain panes SHALL
NOT offer Chat or Trace and opening them SHALL NOT replace the saved agent mode.
The initial pane-agent default SHALL remain Terminal. Expanding a tile or moving to
an adjacent agent SHALL preserve the current supported mode. Plain-pane titles SHALL
remain consistent between roster and focus. Diff SHALL remain available from panels and focused views.
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

### Requirement: User prompt markdown

Browser Chat SHALL render user prompts with the same markdown renderer as replies in both the single-pane view and workbench tiles. The transcript source SHALL remain unchanged. HTML SHALL be rendered as text and actionable links SHALL be limited to HTTP(S) and mailto.

#### Scenario: Read a formatted user prompt

- **WHEN** a user prompt contains headings, emphasis, lists or fenced code
- **THEN** both browser Chat surfaces display its formatting inside the user bubble without executing HTML

### Requirement: Inspectable browser activity

Focused Chat SHALL show activity using the phone's operation names, neutral icons and bounded previews. A delegation preview SHALL prefer its recorded purpose. A wait preview SHALL label only known millisecond settings, without implying elapsed time or completion. The group SHALL show its call count once. A step with additional recorded input, directory or truncation SHALL offer a native disclosure; a summary-only step SHALL remain a plain row. Input SHALL remain plain text, independently copyable and scrollable in a bounded keyboard-focusable region. Live updates SHALL retain expansion, focus and input scroll. A borderless Collapse all action SHALL be available beside the conversation status. Workbench tiles SHALL share the focused Chat activity renderer.

#### Scenario: Inspect a long recorded command

- **WHEN** the reader opens a command row in focused Chat
- **THEN** its recorded command and working directory are available without expanding page width, and Copy copies the record unchanged
- **AND** a subsequent transcript update retains the row expansion, input scroll and keyboard focus

#### Scenario: Read a delegate or wait call

- **WHEN** recorded parameters include a delegation purpose or an explicit millisecond wait setting
- **THEN** the row shows a short purpose or labelled setting, while its inspector preserves the original parameters
- **AND** unknown, oversized or partial parameters are not interpreted as a successful outcome

### Requirement: Consistent browser clipboard actions

Copy SHALL use an icon-only control with an accessible name; Quote SHALL NOT be offered. They SHALL remain discoverable by keyboard and touch without covering reply text. Browser copy paths SHALL share a fallback when the secure Clipboard API is absent, rejects or throws; the fallback SHALL restore focus and selection. Feedback SHALL report success only when a copy operation succeeds.

#### Scenario: Copy on a plain HTTP connection

- **WHEN** the browser has no secure Clipboard API
- **THEN** Copy attempts the browser's legacy clipboard operation, preserves keyboard focus and selection, and reports its actual result
- **AND** no message or terminal key is submitted

### Requirement: Readable workbench tile controls

A narrow workbench tile SHALL wrap its header controls as a group while retaining single-line mode labels, the pane identity and input capability. Header controls SHALL stay inside the tile at supported widths in both languages.

#### Scenario: Resize a compact tile

- **WHEN** the reader uses a 280px or 360px tile in English or Chinese
- **THEN** Chat, Terminal and Diff remain legible without splitting words or characters, and the input-capability chip remains visible

### Requirement: Viewing a question reveals the picker

In a focused Web conversation, an accepted revision-bound View question action SHALL open the in-app terminal without submitting the draft, choosing an answer or changing the saved Chat preference. Expired, denied and uncertain controls SHALL retain the shared input lock.

#### Scenario: Working session has a folded question

- **WHEN** the user taps View question in Chat
- **THEN** only that typed action is sent and the terminal picker becomes visible
- **AND** the unsent composer text remains intact at both narrow and desktop widths

### Requirement: Browser conversation workspace
The browser SHALL present prompts, chronological assistant segments, and expandable tool summaries consistently in focused and workbench chat. Replies SHALL show the turn's agent identity, supervisor sessions SHALL carry the HQ role mark, and text SHALL be copyable on pointer and touch devices. Conversation input SHALL use the existing scope-gated send path and remain intact during transcript refreshes.

#### Scenario: Continue a conversation
- **WHEN** an owner opens a conversation and sends a follow-up
- **THEN** the browser submits to that pane, preserves an unsent draft during polling, and retains readable spacing between messages

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

### Requirement: Local browser entry
The menu bar SHALL offer a local browser workspace at 127.0.0.1, authenticated through a short-lived one-time code. If no serve is available it SHALL start only a loopback listener. It SHALL not enable a tunnel or rewrite an existing remote service.

#### Scenario: Open without remote access
- **WHEN** the user opens the local workspace with remote access off
- **THEN** a loopback-only serve starts and the browser redeems a one-time code for its own authenticated credential

### Requirement: Browser appearance preference
The browser SHALL offer System, Light and Dark appearance modes, default to System, persist an explicit choice per browser origin, apply the resolved mode before first paint, and react to system appearance changes only while System is selected. The workspace, conversation, trace, controls and overlays SHALL adapt; terminal captures SHALL retain their recorded terminal palette.

#### Scenario: System appearance changes
- **WHEN** System is selected and the operating system changes to dark appearance
- **THEN** browser chrome updates without reloading or losing drafts

### Requirement: Closeable trace details

The trace inspector SHALL be closeable. Closing it SHALL give the event list the full
available width and allow longer previews. Closing SHALL survive polling and filtering;
selecting an event or choosing Show details SHALL reopen it. Width-appropriate compact
views SHALL drill into details and return to the same selected row and focus, rather
than dividing an already narrow reading area. Identifiable records SHALL retain their
selection across history additions. Selected text and focused detail controls SHALL
hold their displayed revision during updates to the SAME event and resume after release. If the selected event disappears when a session is replaced, the inspector SHALL immediately show the newly selected event rather than retain the old session under the new list. Inspecting or
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

#### Scenario: Session changes while an old call is selected
- **WHEN** the current transcript replaces the selected event and its old source text remains selected or focused
- **THEN** the inspector immediately displays the newly selected event with matching session metadata on desktop and compact layouts
- **AND** ordinary streaming updates to an existing selected event still preserve the reader's selection

### Requirement: Recognizable browser identity
The browser SHALL provide a bundled gtmux favicon and a localized contextual tab title for the workspace, access page and focused conversation modes. Workspace branding and navigation SHALL provide readable size and spacing on desktop and narrow screens.

#### Scenario: Identify a conversation tab
- **WHEN** a user opens Trace for an agent
- **THEN** the tab title identifies that conversation, Trace and gtmux, and shows the bundled favicon

#### Scenario: Session tool title restores its underlying context
- **WHEN** Trace or Diff opens from focused Chat, Terminal or a workbench panel
- **THEN** the browser title identifies the session and active tool
- **AND** closing the tool restores the underlying conversation mode or workspace title without changing presentation or input

### Requirement: Compact message actions
Conversation messages, tool summaries and trace details SHALL offer a Copy icon with a localized tooltip, accessible name and brief success/failure feedback. They SHALL not offer Quote. Touch targets SHALL remain usable and icons SHALL adapt to appearance.

#### Scenario: Copy a message using its icon
- **WHEN** the reader activates Copy by pointer, touch or keyboard
- **THEN** text reaches the clipboard and the icon reports the actual result without changing to a text button

### Requirement: Consistent browser controls
The browser SHALL use a shared icon system for action controls, with 20px action glyphs, at least 36×36 CSS-pixel desktop targets and 44×44 coarse-pointer targets. Compact tiles SHALL preserve the same target metrics. Action controls SHALL have localized accessible names, visible focus and hover feedback. Layout and appearance disclosures SHALL expose expanded state, support Escape dismissal and return focus. Session rows and authorized reply choices SHALL support keyboard activation.

#### Scenario: Use compact controls by keyboard or touch
- **WHEN** the reader uses a tile's navigation, message actions or composer keys
- **THEN** controls remain legible, touch targets retain their size and authorized actions can be reached by keyboard

### Requirement: Message footer and truthful clipboard feedback
Conversation actions SHALL appear outside the message background in a separate footer. Recorded prompt timestamps and sender attribution SHALL appear in that footer; absent or invalid times SHALL be omitted. Clipboard actions for messages, pinned prompts, pane commands and terminal captures SHALL indicate the actual copy result, including failure of the fallback copy path.

#### Scenario: Clipboard access fails
- **WHEN** both the clipboard API and fallback copy fail
- **THEN** the interface reports failure rather than success and preserves keyboard focus

### Requirement: Consistent browser interface language
Browser routes and overlays SHALL use concise localized action labels, factual empty states and explicit recovery messages. Chinese copy SHALL distinguish the workspace, panels, agent conversations and tmux panes consistently. Agent transcripts and literal commands SHALL remain unchanged. Settings SHALL distinguish conversation text size from terminal font/size controls, with immediate and persisted effects on their respective surfaces.

#### Scenario: Read an empty layout menu
- **WHEN** no layout has been saved
- **THEN** the menu states that no saved layouts exist and offers a clearly named Save current layout action

### Requirement: Collapsible working conversation preview
Chat SHALL show a collapsed-by-default Live strip while the current agent is working, shared by focused conversations and workspace panels. Expanding SHALL show the latest 14 non-trailing-blank rows from the current screen capture, not an earlier transcript reply. The strip SHALL retain its disclosure preference per pane across polling and mode switches, preserve drafts and reading position, and disappear when work stops. Runtime SHALL use the recorded current-state start and server clock when available, and be omitted when unknown. Capture errors SHALL identify unavailable or stale output and retry. Late responses SHALL not appear in a different pane, mode, turn or credential scope, and revoked access SHALL clear cached Live content.

#### Scenario: Watch an active turn
- **WHEN** the reader expands Live during a working conversation
- **THEN** current output updates without replacing transcript history or changing the draft, and the strip closes only when the reader chooses or work stops

#### Scenario: Switch sessions during a slow capture
- **WHEN** a capture response arrives after the reader switches to another session
- **THEN** that response cannot populate the new session's Live content

### Requirement: Shared browser composer parity
Focused conversations and workspace panels SHALL share app-style composition: an upward-arrow Send control, terminal keys ordered Tab, Up, Down, Enter, Backspace, Ctrl-C and Esc, and expandable text entry on touch screens. Enter SHALL insert a newline by default; a persisted Return sends preference MAY enable sending. Modified Enter SHALL send, while IME confirmation and key repeats SHALL not submit accidentally. A full-size editor SHALL preserve the same draft. Controls SHALL remain reachable at narrow/short viewports and adapt to visual viewport and safe-area insets.

#### Scenario: Compose on mobile
- **WHEN** the reader opens the input field or expanded editor on a touch screen
- **THEN** it has 44px controls, multiline text, a separate Send action and reachable controls above the reduced viewport

### Requirement: Staged browser attachments
The composer SHALL support images, files, clipboard images/text, drag-and-drop and mobile camera selection through browser capabilities. Selection SHALL stage attachments without uploading or sending. Photos SHALL offer annotation with draw, arrow, box, opaque redaction, colors, undo and clear. Export SHALL retain original dimensions; a smaller export SHALL require an explicit choice after failure. Send SHALL upload staged files with progress and submit one combined message only after every upload succeeds. Upload failures SHALL keep text and attachments, identify size-limit refusals and allow explicit retry. Selection and asynchronous clipboard results SHALL remain bound to their pane and credential.

#### Scenario: Upload fails before send
- **WHEN** a staged file upload is refused or interrupted
- **THEN** no partial message is sent, the draft and files remain available and the error offers an appropriate recovery

### Requirement: Browser draft and recall state
Text drafts SHALL persist per pane in credential- and server-scoped browser storage. Input history SHALL group by project/session and saved phrases SHALL be editable. Recall SHALL insert text for review without sending. Credential revocation SHALL clear private composition state and close related dialogs. File bytes MAY remain in memory only; after a reload the interface SHALL identify attachments that need reselection and retain any unconfirmed-delivery warning.

#### Scenario: Recall a prior message
- **WHEN** the reader selects history or a saved phrase
- **THEN** the text enters the draft without dispatching and another pane or credential cannot inherit that draft implicitly

### Requirement: Pending prompt feedback and retry identity
After Send, Chat SHALL show a temporary prompt with sending/delivered state until a newly recorded matching occurrence arrives. A pre-existing identical prompt SHALL not confirm a new send. Pending prompts SHALL remain scoped to the pane and credential. An unchanged retry SHALL reuse its send identifier; a new message after acknowledgement SHALL receive a new identifier.

#### Scenario: Acknowledged prompt reaches the transcript
- **WHEN** a newly recorded occurrence matches the pending prompt
- **THEN** the temporary row merges into the recorded conversation without duplication

### Requirement: Speaker identity sheets

User, sender and agent avatars in conversations SHALL be keyboard/touch-activatable
and open an adaptive identity sheet using the app's avatar and information hierarchy.
Sheets SHALL distinguish humans, HQ and agents using the App's identity copy and recorded-fact hierarchy, including shared human quotation and fact ordering. Unrelated browser permissions or current environment information SHALL NOT replace that hierarchy.
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

### Requirement: Session tools preserve the presentation mode

Chat and Terminal SHALL be presentation modes. Trace and Diff SHALL be available
as direct actions beside, but visually separate from, the mode switch in panels and focus. Modes and actions SHALL stay below session identity across viewport sizes. Font size SHALL have one entry in Settings. A tool SHALL retain the
underlying mode, draft and reading position, and retire pending reads on close or
identity/access changes. Readable external sessions SHALL offer Trace without
Terminal, Diff or input. Focused Chat SHALL retain the normal conversation layout.

#### Scenario: Inspect changes from focused Chat

- **WHEN** a reader opens Diff from focused Chat and closes it
- **THEN** the original conversation, selected mode and unsent draft remain
- **AND** Trace is available directly beside the presentation switch

### Requirement: Workspace reading hierarchy

The actual supervisor role SHALL have a distinct pinned section above ordinary
session status groups. Its avatar SHALL use the canonical three-cell gtmux brand mark and an independent status badge. Empty workspaces SHALL offer a usable session-picker action
and a secondary context-specific, attributed excerpt. Native select controls SHALL
retain 44px height, inset arrows and visible focus. Tool summaries SHALL be compact
and unboxed; records without additional input SHALL NOT open empty inspectors.

#### Scenario: Begin from an empty workspace

- **WHEN** the board has no open panels
- **THEN** a clear session-selection action opens the existing searchable picker
- **AND** the atmospheric quotation remains secondary to that action

### Requirement: Authenticated conversation attachment preview

Browser conversation links SHALL reuse the App document reference and metadata
validation contract and owner-only artifact endpoints. An explicit click SHALL load
bounded, digest-verified bytes. Images SHALL have bounded dimensions before decoding.
Closing or losing access SHALL retire reads and release private object URLs. Raw
HTML and arbitrary external resources SHALL NOT execute through the preview.

#### Scenario: Reject corrupted image bytes

- **WHEN** image content differs from its verified metadata digest or size
- **THEN** the preview shows a recoverable failure without decoding that content

### Requirement: Trace opens latest and preserves reading anchors
Trace SHALL open at the newest loaded record regardless of roster status, without a Follow latest mode toggle. Scrolling away SHALL pause implicit tail following; returning to the tail or using a one-shot latest action SHALL resume it. Refresh and loaded history additions SHALL preserve the visible record and its pixel position, including across rendered page boundaries. Explicit range, keyboard, filter and page navigation SHALL take precedence over refresh anchoring.

#### Scenario: Read history during updates
- **WHEN** the user reads an older visible record while new turns or earlier history arrive
- **THEN** its visible pixel position is retained and the inspector does not reopen itself
- **AND** a one-shot latest action remains available without consuming toolbar space

### Requirement: App-aligned session list sections
Desktop and mobile browser session lists SHALL use the App's status glyphs and secondary row information. A recorded error SHALL override the completed badge and error/background text SHALL replace the normal location line. Every populated roster section, including HQ and external sections, SHALL expose a counted, keyboard-accessible collapse/expand control. Choices SHALL persist through polling and reload; collapsed rows SHALL not participate in radar keyboard navigation.

#### Scenario: Inspect an errored idle session
- **WHEN** an idle session has a recorded error
- **THEN** both browser list entrances show the amber error badge and error text instead of a green completion tick and pane-only secondary text

#### Scenario: Keep a section collapsed
- **WHEN** a section is collapsed and its rows change or the page reloads
- **THEN** the section remains collapsed with its updated count and an explicit expand action

### Requirement: Scoped typography and touch input tools
Settings SHALL group display, conversation and terminal preferences with labeled touch-sized controls and viewport-bounded scrolling. Conversation prose and exact focused/tiled terminal sizes SHALL update immediately and survive reload. Reset SHALL restore defaults. Extra terminal keys and input tools SHALL appear for primary coarse/touch input and remain hidden for desktop pointer/keyboard input, independently of viewport width. Composer, attachments, send and drafts SHALL remain available across input-device changes.

#### Scenario: Adjust text while reading
- **WHEN** the reader changes conversation or terminal size
- **THEN** the matching rendered text changes without reload and retains that setting after reload
- **AND** a terminal tile uses the same selected size as the focused terminal

#### Scenario: Resize a desktop window
- **WHEN** a desktop browser window becomes narrow
- **THEN** the touch input-tools row remains hidden and the ordinary composer retains its draft

### Requirement: Direct compact appearance settings
Browser settings SHALL expose System, Light and Dark as immediately selectable choices, without opening a menu. The control SHALL retain native radio keyboard and selection semantics, persisted preferences and system following. Settings SHALL avoid redundant singleton headings and keep labels, size sliders and short hints compact while preserving touch targets and viewport bounds.

#### Scenario: Switch appearance in one action
- **WHEN** a reader activates a visible appearance choice
- **THEN** the page applies it immediately, exposes its selected state and preserves it after reload
- **AND** selecting System resumes following system appearance

### Requirement: Compact readable conversation activity
Browser conversation activity SHALL avoid stacked outer row padding around its disclosure targets. Call rows SHALL retain at least 44px touch targets, readable text/icons and full recorded-input expansion while reducing redundant gaps between messages, copy controls and turns. Separate turns SHALL remain visually distinct, and external conversations SHALL use the same rendering rules.

#### Scenario: Read a long expanded call group
- **WHEN** a reader expands a group of recorded calls
- **THEN** each single-line call uses one compact touch-sized row and opens its full recorded input without changing the conversation or sending input
