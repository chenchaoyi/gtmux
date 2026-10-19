## ADDED Requirements

### Requirement: Versioned terminal key events

The server SHALL expose a shared, versioned catalog of canonical keys and supported
modifier combinations scoped to a visible pane. The new typed key-event form on
`POST /api/send` SHALL preserve existing valid legacy requests. New key/action events
SHALL require a send ID and server/pane context, and SHALL NOT be combined with text,
an appended Enter or another input form. Unsupported events SHALL NOT become text.

#### Scenario: Function key and modifier
- **WHEN** an authorized client sends an advertised Ctrl+F2 event for its current pane context
- **THEN** the shared core encodes one terminal event for that pane without an additional Enter
- **AND** the receipt describes terminal delivery, not execution by the agent

#### Scenario: Unknown key or mixed input
- **WHEN** a client supplies an unknown key, invalid modifiers or more than one input form
- **THEN** the request is rejected before copy-mode exit or any terminal write

#### Scenario: Old server
- **WHEN** a new client cannot obtain the versioned key capability from an older server
- **THEN** it retains only the existing legacy controls and does not try new events or text fallbacks

### Requirement: Key input retains pane grants and bounded retry semantics

New key/action input SHALL use the existing host consent, current link-specific pane
input grant and grant-freshness checks. It SHALL revalidate server/pane identity
before injection and deduplicate concurrent or repeated IDs within a bounded, stated
receipt horizon by caller, boot, binding and payload. A duplicate receipt SHALL NOT
bypass a revoked grant. Uncertain events SHALL NOT be replayed after context/boot
changes or expiry. The system SHALL NOT claim exactly-once delivery across crashes.

#### Scenario: A different share link has permission
- **WHEN** a guest without its own current input grant sends a function key to a pane that another link may control
- **THEN** the request returns 403 and performs no terminal operation

#### Scenario: Concurrent duplicate
- **WHEN** the same authorized caller concurrently submits the same ID, event and current target context
- **THEN** at most one event is injected and the duplicate receives the retained outcome
- **AND** changing that ID's payload is rejected

#### Scenario: Reconnect after server restart
- **WHEN** the outcome of an event is uncertain and the server boot changes
- **THEN** the client reports the uncertain result and does not replay the event under the new boot

### Requirement: Contextual key actions require current evidence

An agent-specific semantic action SHALL be resolved by the core using current target,
action revision and keybinding evidence. Hooks or read-only structured events SHALL
be preferred where actually supported; a missing/known-broken source MAY use a narrow
core live-screen adapter. Discovery SHALL NOT resume a conversation or create another
controller. A pending asynchronous question SHALL NOT alone imply a blocking status.

#### Scenario: Codex question has disappeared
- **WHEN** a client invokes a formerly offered View question action after its context has changed
- **THEN** the server returns a stale-context refusal without opening queued-message editing or sending a key

#### Scenario: Keybinding cannot be verified
- **WHEN** the core cannot determine the current action binding
- **THEN** no semantic action is advertised, while supported generic keys remain available under their literal labels
