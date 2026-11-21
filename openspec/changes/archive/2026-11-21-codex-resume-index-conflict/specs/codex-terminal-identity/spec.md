## ADDED Requirements

### Requirement: Managed Codex resume indexes survive transient heartbeat loss
Every publisher of a resume location index SHALL hold the shared publication lease. A legacy Codex publisher SHALL preserve a different thread or home's managed index while its bridge incarnation is alive or its exit is uncertain. Receipt absence alone SHALL NOT retire the owner. Expiry or explicit retirement SHALL revoke live authority. A durable retirement marker, a positively ended exact incarnation, or a verified relocation committed at its destination permits normal legacy admission. The publisher MAY inspect the private incumbent receipt across Codex homes for process exit and competition, without granting cross-home live input authority.

#### Scenario: A legacy publisher follows a missed heartbeat
- **WHEN** a legacy writer attempts to publish a different Codex thread while the incumbent managed frontend has missed heartbeats but its bridge still exists
- **THEN** the publication is refused without changing the managed index or consuming a pending intent

#### Scenario: The verified producer encounters an index from another pane
- **WHEN** the original connected publisher still proves its complete terminal incarnation and independent receipts place its thread in this pane and the conflicting legacy thread in a different pane, with no competing frontend owner
- **THEN** under the publication lease it durably preserves the exact conflicting bytes as private history, verifies them again before replacement, and restores its own index without restarting the session or replaying input

#### Scenario: Conflict evidence or preservation is unavailable
- **WHEN** the conflicting index has a managed owner, the evidence is ambiguous, the preserved version has changed, or archive durability cannot be confirmed
- **THEN** recovery leaves the conflicting index in place and reports a typed heartbeat failure
