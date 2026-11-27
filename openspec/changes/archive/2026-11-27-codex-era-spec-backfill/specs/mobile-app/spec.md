## ADDED Requirements

### Requirement: Terminal scrollback retention while reading

The human Terminal viewer SHALL request up to 2,000 lines of pane scrollback even during alternate-screen sessions while keeping the native renderer's 1,000-line safety bound. While the reader is scrolled above the live tail, the native terminal SHALL hold the frozen frame until the reader returns to the bottom instead of snapping back on a timer.

#### Scenario: Reader scrolls up in an active terminal
- **WHEN** the user scrolls above the live bottom of a streaming terminal pane (including up to 2,000 lines of captured scrollback rendered within the 1,000-line native bound)
- **THEN** the view stays frozen on the user's history instead of snapping back on a short timer
- **AND** scrolling back to the bottom immediately flushes the latest snapshot and resumes live follow

### Requirement: Amber cached-list freshness notice and bounded manual retry

When a session-list refresh fails or times out while same-client sessions remain cached, the app SHALL display an amber freshness notice with Retry kept available during background reads. An explicit pull or Retry SHALL show busy loader feedback and settle when its requested read completes or 15 seconds from the tap expires without cancelling shared background reads, while transport and access failures keep their red tint and permission boundaries.

#### Scenario: Cached session list fails to refresh
- **WHEN** a session-list read fails or times out while a prior same-client snapshot is displayed
- **THEN** cached sessions remain visible under an amber **Session list is out of date** / **Saved sessions · updated …** banner with Retry available
- **AND** tapping Retry shows **Updating session list…** with the shared brand loader for at most 15 seconds for that tap token (even if queued behind a background read) without cancelling shared background or trailing refreshes

### Requirement: Per-thumbnail attachment upload status and guarded manual retry

Staged composer attachments SHALL display upload state directly on each 72pt thumbnail without a 0% overlay or separate progress row. A failed upload SHALL expose an explicit **Retry sending** action through the guarded send path that retains caption and staged attachments, respects send availability, and never auto-retries. Upload completion SHALL reject empty or whitespace-only saved paths.

#### Scenario: Attachment upload fails and the user retries explicitly
- **WHEN** a staged 72pt thumbnail (showing queued clock, indeterminate brand loader, or determinate ring) fails upload and the user taps **Retry sending**
- **THEN** retry runs through the guarded send path keeping the existing caption and staged attachments without automatic background retries
- **AND** if sending is currently unavailable, the retry respects that send guard

#### Scenario: Upload response returns a blank saved path
- **WHEN** an attachment upload response contains an empty or whitespace-only saved path
- **THEN** the client rejects the upload result as an error instead of staging or sending an invalid attachment path
