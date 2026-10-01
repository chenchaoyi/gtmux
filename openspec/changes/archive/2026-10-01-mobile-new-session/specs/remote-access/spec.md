## ADDED Requirements

### Requirement: Owner creates a detached session remotely

The server SHALL accept authenticated owner `POST /api/sessions` with an optional name and a required request ID. It SHALL refuse guests, including guests allowed to type. It SHALL reject unknown fields, oversized payloads, invalid IDs and control characters. It SHALL create a detached default shell in the Mac user's home and return the real session, pane ID and location without activating a desktop terminal. Existing-name conflicts SHALL be explicit.

#### Scenario: Paired owner creates a session
- **WHEN** an owner submits a valid unused name and request ID
- **THEN** the response identifies the new detached shell and the action log records the authenticated actor, request ID and pane

#### Scenario: Guest cannot expand their scope
- **WHEN** a guest with terminal input permission requests a new session
- **THEN** the server refuses without creating a session

#### Scenario: Retry recovers a live session
- **WHEN** the same request ID and canonical name are retried after a lost response or serve restart while the tagged session is alive
- **THEN** the same session is returned, including a live pane, and no additional session is created by that serve process

#### Scenario: Changed arguments or existing name
- **WHEN** a request ID is reused with a different canonical name, or a new request uses an existing session name
- **THEN** creation is refused with `request_changed` or `name_exists`, respectively

#### Scenario: Receipt lookup fails
- **WHEN** existing receipt lookup fails for a reason other than an absent tmux server
- **THEN** the failure is returned instead of proceeding to create another session

