# conversation-documents Specification

## Purpose
Provide explicitly published server-local conversation documents to paired devices, and support adjustable rich-text selection without exposing arbitrary files or interrupting agent work.

## Requirements

### Requirement: Explicit immutable conversation documents

The system SHALL privately snapshot explicitly published documents and, on an owner/full-scope click, supported local Markdown links verified in the public transcript of the exact conversation/home. It SHALL NOT require a separate publication step for those links, including historical links. Code examples, tool arguments, raw HTML and unlinked paths SHALL NOT grant source access. Files SHALL be owned regular files, with protected locations and unsafe links refused. Limits SHALL remain 30 MiB per file, 240 MiB total and 256 files. Fixed file references SHALL remain immutable; opening a verified source path SHALL refresh its current bytes. Missing sources MAY use an existing authorized snapshot. Local list/remove SHALL affect copies only; a removed version SHALL NOT be silently recreated by later remote clicks.

#### Scenario: Replace a report at the same source path

- **WHEN** a different revision is explicitly published for the same conversation
- **THEN** its fixed reference identifies the new bytes and the original reference stays immutable
- **AND** a registered absolute-path reference resolves the newest published revision

#### Scenario: Guest attempts to read a document

- **WHEN** a guest or unauthenticated caller requests metadata or content
- **THEN** authorization fails before any document lookup or source access

#### Scenario: Open an existing local conversation link without a publication command

- **WHEN** an authorized paired device selects a supported local file link in the conversation
- **THEN** the server verifies the public Markdown destination and exact conversation/home before reading the source and creates a private snapshot automatically
- **AND** a changed source path opens its latest bytes while an existing fixed file reference stays unchanged

#### Scenario: A forged source or removed snapshot is requested

- **WHEN** the requested path is absent from the verified public links, exists only in code/tool details, belongs to another identity or identifies a removed version
- **THEN** the source is not newly published and a removed version is not resurrected

#### Scenario: A Markdown report references a local image or another document

- **WHEN** a paired viewer resolves a reference through a verified Markdown parent
- **THEN** the server verifies the destination in the parent snapshot and requires the same session/home and a file within the parent directory tree
- **AND** symlink escapes, undeclared files, foreign parents and removed parents fail before a source snapshot is provided

### Requirement: Isolated native preview

The iOS app SHALL immediately present loading/progress/error UI, download through the
paired connection without redirects, verify size and SHA-256, and preview the verified
copy. HTML SHALL default to script-disabled static rendering, block external resource
loads/navigation, and have no application bridge or pairing credentials. Enabling scripts
SHALL require native user confirmation for the verified hash in this viewer. Interactive
mode SHALL disclose that scripts may attempt networking and SHALL NOT promise universal
network isolation. Retry, changed content or a new viewer SHALL require fresh consent.
PDF/images SHALL use read-only native preview; text preview SHALL be bounded. Markdown SHALL default to semantic formatted preview with headings, emphasis, lists, block quotes, fenced code and tables, responsive light/dark styles, and a source toggle. Sharing SHALL export the original Markdown. Local images SHALL use opaque native resource IDs and the authenticated parent-document resolver, never credentials in page content. Remote image loads and file scripts SHALL remain disabled. Images SHALL be bounded to 32 references, two concurrent reads, 8 MiB each and 20 MiB total, with pixel checks before decoding. Close
SHALL cancel the request and remove only that viewer's temporary copy.

#### Scenario: HTML report has inline charts and remote requests

- **WHEN** a verified self-contained HTML report is opened
- **THEN** static content renders without executing file scripts, including WebRTC probes
- **AND** inline script rendering begins only after native confirmation, while normal remote subresources, new windows and navigations remain blocked
- **AND** disabling interaction or reopening the file restores static mode
- **AND** sharing explicitly exports the original file through the system share sheet

#### Scenario: File or capability unavailable

- **WHEN** the file is missing, outside the conversation scope, removed, permission is denied, or an old server lacks the endpoint
- **THEN** an appropriate localized explanation and original reference remain available
- **AND** the explanation wraps without truncation and can scroll at accessibility text sizes
- **AND** recoverable failures offer Retry, denied access offers return to the conversation, and a verified unsupported format offers sharing the original
- **AND** the toolbar does not offer Share before a verified original is available

#### Scenario: Read a Markdown report and inspect its original source

- **WHEN** the user opens a Markdown file on phone or iPad
- **THEN** formatting and authorized local images appear in a scrollable responsive preview without executing scripts
- **AND** the user can switch to source and back, share the unchanged original, and close to cancel resource work and remove only viewer-owned caches

### Requirement: Adjustable rich-text selection

The iOS conversation SHALL render text and its selection with the same native layout,
support native range handles across adjacent prose paragraphs, preserve rich links and
full-reply Copy, and pin selected text and link callbacks while new content streams.
Code, tables and tool controls SHALL remain separate selection scopes.

#### Scenario: A reply grows while the user selects text

- **WHEN** a user selects a substring and new reply content arrives
- **THEN** selected text and its link targets stay stable, automatic follow pauses, and
  clearing selection reveals the latest content without pausing the agent

#### Scenario: Older native binary or another platform

- **WHEN** the new native renderer or preview adapter is absent
- **THEN** existing text/copy remains usable and unsupported previews explain the limitation

### Requirement: Stable conversation text layout during refresh
The iOS conversation SHALL initially constrain prose to its available container width rather than a one-pixel placeholder. It SHALL retain the last accepted height during content refresh and reuse a short natural width only for unchanged text layout. Changed prose SHALL regain the available width. It SHALL ignore stale measurements, avoid publishing constrained sizes before a usable width exists, and avoid rebuilding identical native documents. Unbounded code sizing and native selection SHALL remain available.

#### Scenario: Polling and delayed measurements
- **WHEN** a visible conversation refreshes while native size events are delayed
- **THEN** existing prose retains its accepted height and native view identity
- **AND** newly mounted prose starts at the available container width rather than briefly displaying only its first character

#### Scenario: Unresolved and changed container width
- **WHEN** constrained text has no usable layout width, or its container is resized
- **THEN** an unresolved width cannot publish a collapsed single-line height
- **AND** a usable new width remeasures the full text while unbounded code can still report its natural width

#### Scenario: A short message grows
- **WHEN** a one-character message grows into a full paragraph
- **THEN** it regains the available constrained width immediately while retaining its last height until remeasurement
- **AND** a source/link revision with unchanged text layout can retain the measured natural width
