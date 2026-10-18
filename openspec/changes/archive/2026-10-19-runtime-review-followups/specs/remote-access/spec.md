## ADDED Requirements

### Requirement: Own the local listener before publishing serve identity

Foreground serve and tunnel-owned serve SHALL acquire the same non-expiring per-user/port kernel lease and bind before creating or advertising credentials, enrollment codes, status or background API work. Failure SHALL leave an existing owner's status and saved credential unchanged. Process exit SHALL release the lease without unlinking its lock target. SSE work SHALL end with the serving listener. Tunnel reuse SHALL require a ready gtmux health response and acceptance of the existing saved owner credential; an occupied foreign port SHALL NOT be advertised as ready.

#### Scenario: A second launch uses another bind address

- **WHEN** another process holds the same user's port lease
- **THEN** the contender fails before publishing identity or creating a credential even if its requested bind differs

#### Scenario: A tunnel encounters an unrelated service

- **WHEN** the port is occupied by a foreign service or the saved owner credential is missing or rejected
- **THEN** tunnel startup reports that the local radar is unavailable instead of minting a new credential or claiming readiness
