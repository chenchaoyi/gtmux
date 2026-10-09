## ADDED Requirements
### Requirement: Owners can read verified desktop conversations
The system SHALL resolve only a verified desktop Codex conversation ID to bounded public transcript turns. Owner and paired-device reads SHALL be allowed without enabling HQ follow. Guest requests SHALL be rejected before reading a transcript. Unknown or non-desktop identities SHALL be rejected; unavailable or failed reads SHALL not masquerade as an empty completed conversation. Reading SHALL not change follow, notification or knowledge permissions.

#### Scenario: A status-only conversation is opened
- **WHEN** its owner opens a verified desktop conversation whose HQ follow is off
- **THEN** prompts, public commentary, tools and answers can be read without opting into HQ follow

#### Scenario: A guest tries an arbitrary session
- **WHEN** a guest calls the desktop transcript endpoint
- **THEN** no transcript dependency is invoked and the request is forbidden

### Requirement: Desktop conversation readers update safely
The first-batch phone, iPad and native Mac readers SHALL update public conversation content while open, including intermediate replies before a final answer. Reads SHALL be serial, stop when closed or inactive, and ignore responses belonging to a prior session/server. The HTTP reader SHALL use conditional revisions covering all resumed rollout files. A transient failure SHALL preserve already displayed history and expose a retry/error state. Readers SHALL provide no terminal, composer, approval action or adoption action. Follow settings SHALL remain a separate affordance. Web remains a status diagnostic surface for these conversations in this batch.

#### Scenario: A continuation publishes commentary
- **WHEN** a resumed rollout adds public assistant text during a running turn
- **THEN** the next reader update includes it without waiting for turn completion

#### Scenario: A slow old request completes
- **WHEN** the user switched conversation or server before that response arrived
- **THEN** the response cannot populate the new reader

#### Scenario: The app is backgrounded or reader closed
- **WHEN** the reader stops being active
- **THEN** scheduled reads stop and outstanding responses cannot update its closed state
