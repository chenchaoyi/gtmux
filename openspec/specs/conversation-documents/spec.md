# conversation-documents Specification

## Purpose
Provide explicitly published server-local conversation documents to paired devices, and support adjustable rich-text selection without exposing arbitrary files or interrupting agent work.

## Requirements

### Requirement: Explicit immutable conversation documents

The system SHALL publish a document only through an explicit local command, snapshot
its bytes privately, bind it to the supplied session/home identity, and enforce a
30 MiB per-file, 240 MiB total and 256-file limit. Remote path resolution SHALL search
only this registry. Local list/remove SHALL affect registered copies, never originals.

#### Scenario: Replace a report at the same source path

- **WHEN** a different revision is explicitly published for the same conversation
- **THEN** its fixed reference identifies the new bytes and the original reference stays immutable
- **AND** a registered absolute-path reference resolves the newest published revision

#### Scenario: Guest attempts to read a document

- **WHEN** a guest or unauthenticated caller requests metadata or content
- **THEN** authorization fails before any document lookup or source access

### Requirement: Isolated native preview

The iOS app SHALL immediately present loading/progress/error UI, download through the
paired connection without redirects, verify size and SHA-256, and preview the verified
copy. HTML SHALL default to script-disabled static rendering, block external resource
loads/navigation, and have no application bridge or pairing credentials. Enabling scripts
SHALL require native user confirmation for the verified hash in this viewer. Interactive
mode SHALL disclose that scripts may attempt networking and SHALL NOT promise universal
network isolation. Retry, changed content or a new viewer SHALL require fresh consent.
PDF/images SHALL use read-only native preview; text preview SHALL be bounded. Close
SHALL cancel the request and remove only that viewer's temporary copy.

#### Scenario: HTML report has inline charts and remote requests

- **WHEN** a verified self-contained HTML report is opened
- **THEN** static content renders without executing file scripts, including WebRTC probes
- **AND** inline script rendering begins only after native confirmation, while normal remote subresources, new windows and navigations remain blocked
- **AND** disabling interaction or reopening the file restores static mode
- **AND** sharing explicitly exports the original file through the system share sheet

#### Scenario: File or capability unavailable

- **WHEN** the file is unpublished, permission is denied, or an old server lacks the endpoint
- **THEN** an appropriate localized explanation and original reference remain available
- **AND** the explanation wraps without truncation and can scroll at accessibility text sizes
- **AND** recoverable failures offer Retry, denied access offers return to the conversation, and a verified unsupported format offers sharing the original
- **AND** the toolbar does not offer Share before a verified original is available

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
