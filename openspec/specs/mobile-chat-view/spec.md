# Mobile Chat View Specification

## Purpose

Render an agent's parsed transcript (see `chat-transcript`, fed by
`/api/transcript`) as a readable conversation on the phone — so a glance at "对话/
chat" tells you what the agent did and said without parsing its raw terminal. It is
the mobile client renderer of the transcript capability; the browser mirror renders
the same data (see `browser-mirror`).

## Requirements

### Requirement: Conversation of turns with markdown

The system SHALL render each turn as a user-prompt bubble followed by the agent's
reply, both rendered as markdown, kept fresh as the transcript grows, with an empty/
loading state when there is no history yet.

#### Scenario: Render a turn

- **WHEN** a transcript turn is shown
- **THEN** the user's prompt and the agent's reply render markdown inside their respective bubbles

#### Scenario: Expand a long prompt

- **WHEN** a long user prompt is collapsed on the phone or iPad
- **THEN** its bounded plain-text preview and existing expand control remain available
- **AND** expanding it renders the complete markdown with selectable text and the chosen font size
- **AND** HQ prompts remain fully visible, and pending sends remain visibly pending

#### Scenario: Untrusted message content

- **WHEN** a prompt contains HTML or a link using an executable or local protocol
- **THEN** it cannot execute HTML or open that protocol; HTTP(S) and mailto links may open externally; local document references use only the authenticated registered-document adapter, never an OS file URL

### Requirement: Multi-segment replies as separate speech bubbles

The system SHALL render a reply's `segments` as separate speech bubbles in
chronological order, with the agent avatar on every text bubble and the
tool steps that ran between texts shown as a collapsible group (tapping expands the
steps) between the bubbles — so interleaved process reads clearly and segment
boundaries are obvious.

#### Scenario: Steps between bubbles

- **WHEN** a turn has text → tools → more text
- **THEN** each text shows as its own bubble and a collapsible "N steps" group sits
  between them, expandable on tap

### Requirement: The chat marks where the conversation broke

The chat SHALL mark where the conversation BROKE, not where the clock changed.

A separator SHALL appear above a turn when it is the first turn carrying a timestamp,
when its calendar day differs from the turn before it, or when it follows the turn before
it by at least a configured gap. A turn meeting none of those SHALL carry no time of its
own, so that a separator means a pause rather than a minute.

The label SHALL be derived from the prompt timestamp and include the date for clarity
(relative "today/yesterday" wording, calendar date otherwise, the year when not the
current year), localized en/zh.

It SHALL be drawn as a separator and not as a line of text: the label centred, with a rule
either side of it, so a reader scrolling back sees the breaks rather than reading clocks.

A turn with no usable timestamp SHALL never produce a separator, and SHALL NOT break the
comparison for the turns around it.

#### Scenario: A working back-and-forth

- **WHEN** turns arrive a few minutes apart through one sitting
- **THEN** no separator appears between them

#### Scenario: Coming back after a pause

- **WHEN** a turn follows the one before it by at least the gap
- **THEN** a separator above it gives the date and time, centred between two rules

#### Scenario: A new day

- **WHEN** a turn's calendar day differs from the turn before it
- **THEN** a separator appears, whatever the gap

#### Scenario: The start of the history

- **WHEN** the first turn carrying a timestamp is shown
- **THEN** it carries a separator, so the conversation has a beginning

### Requirement: Long-press to select and Copy

The system SHALL let the user long-press to select and Copy text in the chat view
(prompt bubbles and rendered reply blocks), matching the copy affordance of the
terminal view.

#### Scenario: Copy a reply

- **WHEN** the user long-presses a reply bubble
- **THEN** text selection + the Copy callout are offered

### Requirement: Chat renders a window of recent turns, not the whole history

The chat view SHALL render only a window of the most recent turns rather than the entire
transcript, and SHALL offer an explicit control to extend that window further back. This
is required because the view mounts every turn eagerly with replies expanded: a real
long-running session has produced thousands of reply bubbles and tool-step rows, whose
combined view count exhausts device memory and terminates the app when the user switches
to Chat. The window SHALL keep the NEWEST turns, since the tail is what the reader opened
Chat to see. When turns are hidden — whether by this window or dropped by the server —
the view SHALL say so and say how many, rather than presenting a partial conversation as
the whole one.

#### Scenario: Opening a long conversation

- **WHEN** the user switches to Chat on a session with far more turns than the window
- **THEN** the most recent turns are rendered and the app remains running

#### Scenario: Reaching further back

- **WHEN** the user asks for earlier turns
- **THEN** the window extends toward the start of the available history

#### Scenario: Truncation is disclosed

- **WHEN** any turns are not being shown, whether windowed away or dropped before
  reaching the client
- **THEN** the view states that earlier history is not shown and how many turns that is

### Requirement: A restarted conversation says where it restarts

When the server reports that a session began by starting the conversation over, and no
turns are being withheld by the window or dropped before reaching the client, the chat
view SHALL say so — naming the command and, when known, the local clock time — instead of
presenting a short history with no explanation. The restart disclosure itself SHALL
offer no control to load more turns from that session: those are all the turns available
for it. Loading a preceding HQ session is a separate action, described below.

Truncation SHALL take precedence over this disclosure: while turns remain loadable or
were dropped, that is what the reader needs to know, and the restart is only the end of
the road once nothing else is hidden.

On the HQ console, the separate "Load the earlier session" control SHALL be offered
when the server reports an earlier session and no turns remain hidden by the client
window. It asks for another session from the HQ audit chain, as specified in
[The HQ console reaches the session before a clear](../mobile-app/spec.md#requirement-the-hq-console-reaches-the-session-before-a-clear).
The retired Activity zone is not a destination for this disclosure. A worker's Detail
has no session-chain control and SHALL omit it.

This exists because a cleared conversation and a broken one look identical: a supervisor
shift that had run for hundreds of turns was cleared and the phone showed three bubbles,
which was read as a bug in the app.

#### Scenario: Opening a conversation that was cleared

- **WHEN** the session began with a `/clear` and nothing else is hidden
- **THEN** the view states that this conversation starts there, names the command and the
  time, and offers no load-more control as part of that disclosure; an available
  preceding HQ session has its own control

#### Scenario: A truncated conversation that also restarted

- **WHEN** turns are windowed away or were dropped by the server
- **THEN** the view discloses the truncation, not the restart

#### Scenario: An ordinary conversation

- **WHEN** the session was not started over and nothing is hidden
- **THEN** the view renders no such line at all

### Requirement: A turn in progress is visible in the conversation

While the agent is working, the chat view SHALL show progress in the conversation itself,
at the tail where the reader is looking, and SHALL state HOW LONG the turn has been
running. A status word alone is not sufficient: it sits above the scroll where the reader
has already passed it, and it cannot distinguish a turn that is thinking from one that has
hung — and only one of those is worth interrupting. When the pane has produced visible
output the view SHALL show that output; when it has produced none yet the view SHALL still
mark the turn as running rather than showing nothing. Where the elapsed time is unknown the
view SHALL omit it rather than present a fabricated duration.

#### Scenario: A long turn with no output yet

- **WHEN** the agent has been working for a while without printing anything
- **THEN** the conversation shows that a turn is running and how long it has been running

#### Scenario: A turn producing output

- **WHEN** the agent is working and its screen has content
- **THEN** that content is shown live at the tail of the conversation, titled with how
  long the turn has been running when its start is known

#### Scenario: The start time is unknown

- **WHEN** the view cannot tell when the turn began
- **THEN** it reports the turn as running without claiming an elapsed time

### Requirement: Full reply actions

Phone and iPad Chat SHALL provide icon-only Copy reply and Share beneath each text-bearing turn, including when folded. Actions SHALL use every textual segment in order as Markdown, falling back to response for older transcripts, and exclude the prompt and tool output. In-progress replies SHALL use only text received so far. Copy SHALL show a temporary checkmark confirmation, and Share SHALL open the system share sheet without selecting a recipient. Actions SHALL retain localized accessibility labels and touch targets at least 44 by 44 points, without visible text labels. Tool-only turns SHALL not show these actions. The unboxed action row SHALL follow the final text segment before any trailing tool calls.

#### Scenario: Copy a folded segmented reply

- **WHEN** the reader copies a reply with hidden text segments
- **THEN** the clipboard contains all of that turn's textual segments in order, not its truncated preview
- **AND** the copy control confirms completion

#### Scenario: Share a reply

- **WHEN** the reader taps Share below a reply
- **THEN** the system share sheet receives the same complete text and the reader chooses the destination

### Requirement: Secondary tool activity opens only on reader intent

Phone and iPad Chat SHALL keep both live and historical tool groups folded by default as a grey summary on the reading canvas. Opening SHALL reveal compact icon-led rows without a separate card background. Only a call with recorded additional input, directory or a partial-detail notice SHALL offer detail expansion. Empty inspectors and repeated tool-name placeholders SHALL be omitted. Original input SHALL remain selectable, horizontally scrollable and copyable; bounded multiline scrolling and partial/summary-only notices SHALL remain. Reader choices SHALL survive updates and turn completion. Commentary and elapsed-time progress SHALL remain visible independently.

#### Scenario: Argument-free wait

- **WHEN** a reader expands a group containing only a Wait without recorded parameters
- **THEN** it shows one non-interactive Wait row, without an empty inspector or detail chevron

#### Scenario: Inspect a real command

- **WHEN** a reader expands recorded long command input
- **THEN** the displayed text can be scrolled and copied without executing it, and closing the group removes its detail views

#### Scenario: Live calls arrive

- **WHEN** calls append to a working turn
- **THEN** the summary updates without automatically opening the list or overriding a deliberate reader choice

### Requirement: Native session trace tools

The native App SHALL expose Trace through a session-tools control, alongside Diff
when available. Chat and Terminal SHALL remain presentation modes. Trace SHALL share
the parent transcript reader without replacing its composer or restarting an active
Chat subscription. HQ and readable external sessions SHALL use the same reader UI;
plain shells SHALL NOT offer Trace. External sessions SHALL remain read-only.

#### Scenario: Inspect without changing mode
- **WHEN** a user opens Trace from Chat and closes it
- **THEN** the mounted conversation, unsent draft and reading position remain
- **AND** opening or closing the tool does not start a duplicate transcript read

### Requirement: Adaptive trace reading

Trace SHALL provide a virtualized list, search over loaded history, type and session
filters, position-based tail following, original-text Copy, Markdown/source and recorded input,
working directory and truncation notices. Details SHALL be closable. Compact views
SHALL preserve the mounted list and return to its selected record; wide views SHALL
use measured content width and font scale to split list/details. Closing details
SHALL expand the list. Absent times and outcomes SHALL NOT be fabricated.

#### Scenario: Freeze a selected revision
- **WHEN** a selected record changes during streaming or history is prepended
- **THEN** its displayed text and historical speaker remain together
- **AND** an explicit update action adopts the latest recorded version

#### Scenario: Retire private records
- **WHEN** client, credential, pane, home or session identity changes
- **THEN** old records, selections and profiles cannot appear in the new scope
- **AND** a delayed old read cannot replace its history

#### Scenario: External read access is revoked
- **WHEN** the external conversation returns HTTP 401 or 403
- **THEN** the cached conversation clears and Trace closes

### Requirement: Shared trace indexing

The browser and native readers SHALL consume one typed pure event model through a
generated browser asset and shared synthetic fixtures. A generation check SHALL
reject stale or manually changed output. Historical profiles and files SHALL use
the selected message identity through the existing authorized readers.

#### Scenario: Compare providers
- **WHEN** Codex, Claude, Kimi, OpenCode or DSH records are indexed by either reader
- **THEN** the same fixture yields the same ordered records, inputs and session boundaries

### Requirement: Bounded native Diff reading

Diff SHALL mount only a window of native rows while retaining access to every returned row and horizontal scrolling. Row metrics SHALL track system text size. Native line measurement SHALL be bounded; any shortened line SHALL have a visible notice. A failed request SHALL show an error and Retry, separately from an empty diff. Closing or switching panes SHALL retire earlier responses.

#### Scenario: Inspect a large working tree
- **WHEN** a diff contains 35000 lines
- **THEN** native row views remain bounded and the user can scroll, close and reopen the tool

#### Scenario: Retry a failed read
- **WHEN** the Diff read fails and the user selects Retry
- **THEN** the error remains distinct from no changes and a successful retry shows the current pane's diff

### Requirement: Compact external conversation identity

Phone and iPad external readers SHALL share a compact header containing one title
line and one harness / read-only line. Tapping the title SHALL reveal the full title,
source, original-client input guidance and recorded connection evidence in a bounded,
scrollable, dismissible details sheet. Secondary evidence SHALL NOT permanently
expand the header or grant input authority. Back and tools SHALL retain independent
44pt targets. Details SHALL close when the server, home, session or authorization
changes, while dismissal SHALL preserve the conversation.

#### Scenario: Read an external conversation on a narrow screen
- **WHEN** a paired owner opens an external conversation with a long title and launch evidence
- **THEN** the header stays two lines and evidence is available by tapping its title
- **AND** closing details returns to the same read-only conversation without sending input

### Requirement: Progress-oriented trace reading
Native Trace SHALL open at the newest recorded activity without a Follow latest mode switch. It SHALL group records by their exact conversation and stable turn identity, provide a latest-turn summary, and compact consecutive tool calls without discarding records. Refresh SHALL preserve a reader away from the tail and the selected detail revision. Near-tail readers MAY continue following new activity.

#### Scenario: Inspect history while the agent updates
- **WHEN** a reader scrolls away from the tail or inspects a historical record
- **THEN** appends, streamed text and loaded earlier history do not pull the viewport away
- **AND** a one-shot latest action can return to the newest activity
