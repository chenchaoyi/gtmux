## ADDED Requirements

### Requirement: DeepSeek Harness terminal clients preserve one harness identity

Supported terminal clients SHALL use the existing `dsh` agent identity and official
DeepSeek branding. Client-specific discovery and input behavior SHALL belong to a
modular adapter. A verified interactive tmux attachment SHALL be a managed DSH pane;
Web conversations SHALL retain their external-history behavior. A process name,
working directory or mounted-session set alone SHALL NOT authorize input.

#### Scenario: A supported terminal client controls a DSH conversation
- **WHEN** its live process, canonical home, current session and tmux attachment are verified
- **THEN** the core exposes one managed DeepSeek Harness conversation across consumers
- **AND** a second external row for the same conversation is suppressed

#### Scenario: A client uses another backend or loses its attachment
- **WHEN** the backend or foreground binding is unavailable, changed or ambiguous
- **THEN** DSH input capability is withheld or revoked without guessing a conversation
- **AND** external saved-history reading remains governed by its existing policy

### Requirement: Terminal input is addressed and acknowledged by the receiving native agent

Submitted DSH text SHALL retain the existing draft and permission safeguards and use
a bounded, one-use native request carrying the expected conversation, home and live
client instance. The receiving command SHALL validate its actual target before
queuing user input. An acceptance receipt SHALL distinguish accepted input from
execution and completion. Retries SHALL NOT create duplicate input, including when
a response is lost. A stale session, foreign request, missing bridge or revoked
client capability SHALL NOT fall back to untargeted text or another writer.

#### Scenario: A session switches during delivery
- **WHEN** the receiving native command targets a different session or client instance
- **THEN** it refuses the request before queuing its text
- **AND** it reports a refusal rather than a successful delivery

### Requirement: HQ message coverage is recoverable and review is explicit

Verified terminal conversations SHALL provide stable public-message identities and
source sequence cursors for user text and assembled assistant text. Private reasoning,
injections, raw tool results and model-surface replacement copies SHALL be excluded.
Live events MAY trigger discovery, but persisted artifacts SHALL reconcile missing
evidence incrementally with bounded memory. Missing or invalid source history SHALL
be visible. Reading or notifying SHALL NOT by itself mark a message reviewed.

#### Scenario: Live event delivery is missed
- **WHEN** a committed public message is absent from the live feed
- **THEN** the next bounded reconciliation recovers it without duplicating prior evidence
- **AND** interruption and failure retain their non-success meaning

### Requirement: Terminal-client setup verifies ownership and runtime capability

Installation and repair SHALL share the provider's writer, preserve foreign files
and configuration and declare only required native component capabilities. Doctor
SHALL distinguish installed configuration from a loaded, verified terminal bridge.
An existing agent process SHALL NOT be restarted merely to make diagnosis healthy.

#### Scenario: A bridge is installed but not loaded
- **WHEN** its owned artifacts are current but no valid runtime attachment exists
- **THEN** diagnosis reports the loading boundary and does not claim control is ready
