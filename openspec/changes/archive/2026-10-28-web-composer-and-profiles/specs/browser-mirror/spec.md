## MODIFIED Requirements

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


### Requirement: Compact message actions
Conversation messages, tool summaries and trace details SHALL offer a Copy icon with a localized tooltip, accessible name and brief success/failure feedback. They SHALL not offer Quote. Touch targets SHALL remain usable and icons SHALL adapt to appearance.

#### Scenario: Copy a message using its icon
- **WHEN** the reader activates Copy by pointer, touch or keyboard
- **THEN** text reaches the clipboard and the icon reports the actual result without changing to a text button

## ADDED Requirements

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
User, sender and agent avatars in conversations SHALL be keyboard/touch-activatable and open an adaptive identity sheet using the app's avatar and information hierarchy. Sheets SHALL distinguish users, HQ and agents, show available role/session/status/source information and offer navigation only to a currently authorized matching session. Missing personal/device information and historical identities SHALL not be replaced with inferred current data. Settings SHALL use an icon with a localized accessible name.

#### Scenario: Inspect a historical speaker
- **WHEN** the speaker does not resolve to a matching current session
- **THEN** the sheet identifies the recorded speaker as historical without assigning another agent's current status or enabling navigation to it
