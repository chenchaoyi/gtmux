## ADDED Requirements

### Requirement: Codex observation must not acquire control
The system SHALL use validated read-only capabilities on trusted existing backends
or verified bounded stored-history reads. Viewing SHALL NOT start a backend, resume,
submit, change permissions, or acquire a writer. Pagination and omissions SHALL be
bounded and explicit. Terminal content SHALL remain actual terminal output rather
than a fabricated transcript rendering.

#### Scenario: Read a locked CLI conversation
- **WHEN** an authorized reader opens history while a CLI owns the thread
- **THEN** it SHALL read public conversation content without acquiring control
- **AND** closing it SHALL leave the writer and grants unchanged

#### Scenario: Backend lacks a safe observation capability
- **WHEN** the protocol version cannot support validated read-only observation
- **THEN** the system SHALL use a supported bounded stored-history fallback or report unavailable
- **AND** it SHALL NOT call resume as a subscription substitute
