# session-restore — delta

## ADDED Requirements

### Requirement: Resume a conversation from the directory it is filed under

When relaunching an agent conversation, `gtmux restore` SHALL resume it from the directory
the conversation is actually filed under, not merely the working directory last observed
for the pane — an agent may change directory mid-session, and resuming from the moved-to
directory fails to find the conversation. For an agent whose transcript store is known
(Claude Code), the system SHALL locate the transcript by session id and take the session's
recorded working directory from the transcript itself rather than decoding the store's
directory name (that encoding is lossy). If no transcript exists for the session id, the
system SHALL SKIP the resume and record why, rather than running a command that can only
report a missing conversation. Agents whose store cannot be inspected SHALL be unaffected.

#### Scenario: The agent changed directory during the session

- **WHEN** a conversation was started in one directory, the agent later changed into a subdirectory, and restore relaunches it
- **THEN** the resume runs from the directory the conversation is filed under, so the conversation is found

#### Scenario: The conversation no longer exists

- **WHEN** a resume record names a session id with no transcript on disk
- **THEN** restore skips that resume and logs the reason, leaving a usable pane instead of an error message
