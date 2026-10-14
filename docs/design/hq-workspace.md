# HQ workspace

## 1. Scope

The menu-bar HQ card opens a native sidebar/detail workspace. Its context is a
**matter**: a durable goal independent of a pane or a supervisor conversation.
Phone/iPad/Web keep their existing HQ interface in this batch; no workspace HTTP
write endpoint or guest/share access is added. The terminal exposes the same local
protocol with `gtmux hq workspace`. Existing board, knowledge, machine and usage
readers retain ownership of their data.

## 2. Information and interaction

The sidebar contains New task, matters grouped by Needs your input / Following /
Recent, Talk to HQ and HQ activity. A matter's header shows its name, explicit state
and associated conversations. Click a conversation to read its original record;
Open in Terminal resolves its exact identity again rather than using a saved pane
number. A missing or ambiguous current pane is an error, while its stored source
can remain readable. The toolbar reuses the board/knowledge/usage readers and offers
an explicit Terminal escape hatch. The HQ card context menu also opens Terminal.

New task requires only an instruction: its first line supplies an editable title.
Global Talk to HQ does not create a matter. The composer labels its scope. Enter
inserts a newline; Command-Return sends. Drafts survive selection/window closure
within the current app process. User Markdown sits right with the mobile
person-battery avatar; HQ public text sits left. Tool steps and journal-confirmed
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
- `HQWorkspaceView/Conversation/Participants/Activity`: composed native surfaces;
  existing Theme, MarkdownBody, AgentIcons, HQMedallion and readers are reused.

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
