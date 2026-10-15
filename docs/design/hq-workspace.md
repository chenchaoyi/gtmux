# HQ workspace

## 1. Scope

The menu-bar HQ card opens a native sidebar/detail workspace. Its context is a
**matter**: a durable goal independent of a pane or a supervisor conversation.
Phone/iPad/Web keep their existing HQ interface in this batch; no workspace HTTP
write endpoint or guest/share access is added. The terminal exposes the same local
protocol with `gtmux hq workspace`. Existing board, knowledge, machine and usage
features share this one window and retain their existing data sources.

## 2. Information and interaction

The first opening shows **Overview**. It and the sidebar use the same explicit matter
records, grouped by Needs your input / Following / Recently ended. No Markdown heading
or pane activity determines matter state. Each sidebar category has a full-row disclosure,
count and accessible expansion value; collapse choices survive navigation and window
reopening in the current process. Matter symbols reuse session status: red pause for
needs-input, brand cyan ring for following, green check for ended, with text labels.
The sidebar also contains New task, Talk to HQ, HQ activity and Resources (Knowledge,
Machine, Usage). The authored board is a secondary **Handoff snapshot** from Overview,
not another current to-do list. It retains the full source, author order and saved time;
sections start collapsed, old asks are not lifted into a red attention block and saved
pane numbers are not active links. It neither rewrites nor migrates the board. A matter's header shows its name, explicit state
and associated conversations. Click a conversation to read its original record;
Open in Terminal resolves its exact identity again rather than using a saved pane
number. A missing or ambiguous current pane is an error, while its stored source
can remain readable. Linked conversation cards show agent, exact current pane/location
and status. A saved locator without a matching current identity is labelled Last pane;
a reused pane cannot supply another conversation’s status. The full session ID is
selectable in the source reader. The duplicate HQ assessment disclosure is removed
from the header; stored assessments and full conversation remain intact. The toolbar
provides refresh and an explicit Terminal escape hatch. The HQ card context menu also opens Terminal.

New task requires only an instruction: its first line supplies an editable title.
Global Talk to HQ does not create a matter. The composer labels its scope. Enter
inserts a newline; Command-Return sends. Drafts survive selection/window closure
within the current app process. User Markdown sits right with the mobile
person-battery avatar; HQ public text sits left with the gtmux mark. Sender names sit
above the bubbles and Copy below each public message preserves its complete Markdown
source, with temporary success/failure feedback. Global, matter and participant-source
readers share the same components. Time separators match mobile: first valid clock,
calendar-day change or at least 15 minutes since the previous valid clock. Unknown times
stay unknown and do not reset that comparison. A session seam does not replace its
turn's public prompt/reply. Tool steps and journal-confirmed
wake batches fold by default. Merely resembling a wake line never hides user prose.
Reading never acknowledges HQ's unread watermark.

Manage conversations adds current or previously recorded tmux conversations,
chooses execution or historical-reference role and edits an explicit time interval.
An ongoing association defaults to now; a historical reference needs an end time.
Transfer/unlink/merge/split change metadata only. Unlinked source records survive;
a merge preserves original message scopes and resolves through explicit edges.
A split moves chosen associations, retains prior discussion in the source and
records a clickable parent link. It does not fabricate a separate past dialogue.
Nothing groups conversations by title, project directory, process name or pane.
Native desktop sources are not added to HQ participation by this feature.

## 3. Modules and protocol

- `internal/hqworkspace`: schema, atomic snapshots, transactions, associations,
  scoped messages, receipts and operation changes. No UI/tmux dependency.
- `internal/app/hqworkspace*.go`: local CLI adapter, source projections, verified
  target lookup and the existing `dispatchbridge`/`dispatch` input verifier.
- `HQWorkspaceModels/Client/Store/Controller`: native decoding, bounded literal
  stdin transport, asynchronous interaction state and window lifetime.
- `HQWorkspaceView/Sidebar/Overview/Composer/Conversation/Participants/Activity`:
  composed native surfaces; shared `HQWorkspacePresentation`, `HQChatBubble`,
  `HQChatTimeline` and `HQBrandLoading` own reading conventions;
  existing Theme, MarkdownBody, AgentIcons and HQMedallion are reused.
- `HQResourcePage/Store/ResourcesView` and feature views (`HQBoardView`,
  `HQKnowledgeView`, `HQMachineView`, `HQUsageView`, `HQMemoryBar`): sidebar-selected
  content, retained knowledge search/selection, bounded reads and archive actions.
  No legacy reader controller or second HQ window remains.

A conversation identity is agent key + configured agent-home ID + session ID.
Locator and name are presentation metadata. Commands accept one strict JSON object
on stdin; no message enters shell interpolation or process arguments. Reused request
ID with identical content converges; changed intent is rejected. Metadata writes
use matter revisions so a stale sheet cannot overwrite another change.

`spawn --matter ID` validates an active matter before launch, carries its explicit ID
in the existing dispatch ledger, and confirms an association only when the dispatched
pane has a binding matching its saved session identity or the exact dispatch nonce. Until then its JSON receipt
says `pending-binding`; the existing serve fast tick reconciles it. Failed dispatches
are not linked. A retry for another matter cannot reuse the original unsubmitted task
by title. Structured `report` writes require the current HQ pane. The v55 managed
playbook teaches IDs, scoped reporting and explicit completion; development does not
rewrite the running HQ's personal records.

## 4. Delivery and failure handling

Create commits the new matter and its first instruction together. Delivery claims a
saved message before typing and holds a process-wide lease through the full input
attempt. Every paste/Enter uses existing draft/state guards and rechecks HQ identity.
Busy/offline/waiting/draft states leave the instruction pending. The serve fast tick
retries only safe holds, never an uncertain or already submitted instruction.
Closing a matter holds its pending instructions; reopening allows delivery again.
Merging redirects still-pending context through its recorded merge edge.

Receipts distinguish pending, submitting, delivered, accepted-queue, uncertain and
cancelled. A receipt describes transport, never task completion. A process lost after
claim or a failure after paste remains uncertain: text is retained and explicit retry
requires acknowledgement of possible duplicate execution. Withdrawal is allowed only
before an attempt. HQ result/Stop/idle cannot complete a matter; completion is an
explicit goal decision. The current UI doesn't answer Terminal permission menus or
interfere with HQ rotation. It keeps the existing Terminal route for these cases.

`workspace/records.json` is private (0600), atomically renamed, fsynced and serialized
by a file lock. Write/durability failures are surfaced, not empty success. Durable
message/matter IDs and the change sequence link actions to receipts; ordinary dispatch
and diagnostic records still preserve their own audit source. Full HQ backups include
these records. Restore marks pending/submitting/queued instructions uncertain before
installing the archive so old instructions cannot replay automatically. Selective
knowledge migration excludes operational workspace records.

## 5. Bounds and acceptance

Workspace message history pages by sequence (60/page). Original participant reads are
an interval-filtered preview of up to 200 recent turns. Raw global HQ history reuses
the existing 300-turn parser, payload budget and predecessor-chain reader (up to 20
requested earlier sessions). Public raw replies over 256 KiB and source turns over
512 KiB remain in original records; the view warns instead of claiming full coverage.
The source-turn response budget is 2 MiB; the native transport rejects over 8 MiB.
Activity shows the latest 200 workspace changes and 200 relevant journal events from
the last seven days, including receive/end/fail, supervision, sends, knowledge and
rotation. Source failures/uncertainty remain visible. These previews are not a full
forensic export, and the runtime does not infer historical matter membership.

Tests use isolated stores/fake dispatch and CLI clients. Native-host layout and
rendering are checked separately from live app/VoiceOver acceptance. A current HQ
command can be affected by the active client's existing identity mapping; the new
layer reuses foreground-agent/home/locator/turn checks, not an invented exclusive
owner detector. It refuses mismatched or ambiguous current links. Future mobile/Web
adapters must enforce owner access and use this source model, not copy heuristics.

Quiet polling compares core data and source revisions before parsing conversation
history again. Source changes still load live public commentary. CLI mutations and
delivery use separate native queues; receipt waiting runs outside the serve fast tick.
The composer reuses the screenshot tool's native multiline input implementation
including input-method and explicit-send handling.

Workspace delivery, automatic HQ wakes, rotation and startup briefing share a nonblocking input lease
(`internal/hqinput`). Competing channels keep their saved queue instead of writing
over one another. Startup briefing retains its existing best-effort refusal and reports
contention. Rotation re-reads its durable attempt after acquiring the lease, so a stale
unsent snapshot cannot repeat a reset. Human typing still relies on existing draft/submit guards.

## 6. Enrollment of existing work

A first workspace read can legitimately have no matter records while the board and
conversations are populated. Readers do not guess goals from Markdown or panes and do
not migrate personal records. The empty Overview explains this and opens the handoff snapshot or
prepares an editable global review request. Preparing it never sends; both scoped and
global drafts survive, and an occupied global draft is shown instead of overwritten.

The v56 managed charter teaches startup reconciliation and maintenance across all
instruction channels. HQ verifies continuing goals within the user’s existing follow scope (excluding native
conversations without HQ-follow consent), reuses existing IDs and uses the
HQ-only `register` operation for already assigned work. Registration atomically saves
explicit state, an assessment, typed source citation and one HQ report. It adds no user
instruction, delivery or association. A retry compares immutable registration fields,
so later rename/state/report changes survive. Invalid/conflicting intent and disk errors
are reported. Source references are displayed as text; they are not executable paths.
Sessions are associated separately through the existing exact identity/interval model.
This is agent-assisted enrollment, not a deterministic Markdown migration or a promise
that an old running HQ client has already reread its new charter.

## 7. Unified Mac destinations

The menu-card overview icon opens Overview; knowledge/machine/usage shortcuts select
that resource in the same retained window. The report's Handoff snapshot row retains
the saved-board age and opens the snapshot directly. Opening HQ without a destination preserves the
current selection. Overview and resources have no instruction composer, and the store refuses
send there. Global, matter and new-task drafts survive resource navigation.

Knowledge keeps search and a responsive list/detail split. HQ archives places
Export and Import here; Import retains Restore backup, Move from another Mac and
Continue staged migration. Size, local snapshots, protection status and last-export
details are available in the archive label’s help. Migration remains selective and
never imports the old machine’s board or operational workspace.

Only the selected resource page polls (20 seconds), through bounded CLI reads and
a gate per destination. Leaving resources, closing or occluding the workspace stops
the timer; already-running reads may finish into their own cache. Failed reads
retain valid snapshots and expose Retry. Memory metadata is read in the background.
Native layout/AX fixture checks and actual VoiceOver speech acceptance are recorded
separately; tests do not establish live accessibility acceptance.

## 8. Reading feedback

An unread value shows a breathing gtmux mark with a short loading label, not an empty
conversation or an offline supervisor. It becomes a retryable failure or valid content
only after the read finishes. Conversation/activity first loads, resource pages and
participant-source readers share this pattern. Refresh keeps valid content and uses a
compact brand mark; quiet polling does not insert a new sidebar row. Reduce Motion
uses a static mark. Existing generation/selection guards keep late results in their own
scope. The loading animation represents a page read, not an agent working state.
