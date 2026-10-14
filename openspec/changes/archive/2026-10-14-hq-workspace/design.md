# Design

## Ownership and reuse
`internal/hqworkspace` owns the persistent value model and transactions. `internal/app`
composes existing radar, resume, transcript, dispatch and event readers into the CLI
report, resolves the verified HQ and supplies the delivery adapter. No import below
app points back to app. Swift has separate models, CLI client, observable store, window
controller, sidebar, detail, composer, conversation, participants and activity views.
Existing readers continue to own board/knowledge/resource/usage; they are doors rather
than duplicated data models. Native views use the established system typography,
status tokens and source identity. UI work remains off the main thread when reading CLI.

## Durable identifiers
A matter has its own ID and explicit state: open, needs-input, completed or closed.
Associations record agent key, agent-home identity, session ID, role, from/until interval,
and last-known locator for display. Locator is never identity. Existing conversations
default to association from now; retrospective reference requires an explicit interval.
Recycled pane numbers, changed conversation ownership and renamed titles cannot redirect
links or attach unrelated history. Spawn receives an explicit matter ID; inheritance
does not use workspace cwd or conversation title. HQ remains coordinator rather than
one worker implicitly attached to every matter.

## Delivery
Persist a caller-supplied request ID and exact message before delivery. Repeating the
same ID/content returns the saved receipt; changed content is refused. Claim one request
transactionally before typing. Bind each attempt to a verified supervisor session and
recheck identity/draft/state immediately before input. Busy, waiting, unavailable or
draft-bearing HQ holds the request; use the existing fast tick to retry only held,
never-attempted requests. A process lost after typing yields uncertain, not automatic
replay. Explicit retry preserves ID and requires evidence/choice when prior submission
is uncertain. A later confirmed HQ session can receive held requests for the same HQ
home; already submitted attempts are never replayed across rotation.
Receipts say pending, delivered, accepted-queue, refused, uncertain or cancelled;
execution progress/result remains a separate matter/update field. All failures remain
visible and preserve the user's text. The ordinary send/driver receipt remains authority.

## Conversation and evidence
Workspace user messages are scoped to a matter or global conversation. HQ can publish
typed updates using the CLI and exact message/matter IDs; full transcript remains a
fallback and source link. Match explicit IDs before associating a turn. Never copy every
HQ turn into every matter. Existing unattributed prose stays in global/raw history.
Render public commentary live, fold tool steps, and group verified supervision signals.
Keep user prose, failures, withdrawn conclusions and real decisions visible.
Bound reads and provide earlier-history pagination without truncating stored records.
Activity is an event-journal projection of HQ receipt/dispatch/maintenance/rotation and
channel failures; reading it does not advance the supervisor's watermark.

## Interaction
New task opens a focused composer without requiring a name, directory or agent choice.
Matter detail shows its goal/state, participants, explicit latest assessment, scoped
left/right conversation and expandable evidence. Composer scope is fixed and labelled;
Cmd-Enter sends, Enter inserts a newline. Drafts are retained per scope. Global HQ
conversation is distinct from New task. Participants are clickable verified conversation
links, with add/reference/move/unlink controls and interval editing; metadata changes
never stop or restart a client. Matter merge/split preserve provenance and history.
Poll only while visible, preserve selection/expanded rows/scroll position, and display
new-message affordance while the reader is above the tail. Native split layout and
toolbar adapt to screen/window dimensions. Retry, empty, loading, unavailable HQ and
old-core compatibility are explicit states, not empty success.

## Acceptance notes
The accepted images are design references, not proof of runtime behavior. Use isolated
stores and fake dispatch IO for mutation/failure tests. Do not send test instructions
to the real HQ. Native hosting-layout checks and CUA runtime/VoiceOver checks are reported
separately; unavailable automation must not be recorded as passed. Disk is about 13GiB
available at start; use one task-owned Swift scratch, no mobile/simulator build.
