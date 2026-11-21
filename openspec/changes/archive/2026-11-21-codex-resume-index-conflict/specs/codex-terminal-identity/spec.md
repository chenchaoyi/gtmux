## ADDED Requirements

### Requirement: Managed Codex resume indexes survive transient heartbeat loss
Legacy Codex publishers SHALL preserve an incumbent managed frontend index for a different thread or home while its receipt is present and the bridge incarnation is alive or uncertain. The expired receipt SHALL remain ineligible for live input. An explicitly retired receipt, a positively ended bridge incarnation, or a verified relocation committed at its destination permits normal legacy admission.

#### Scenario: A legacy publisher follows a missed heartbeat
- **WHEN** a legacy writer attempts to publish a different Codex thread while the incumbent managed frontend has missed heartbeats but its bridge still exists
- **THEN** the publication is refused without changing the managed index or consuming a pending intent

#### Scenario: The verified producer encounters an index from another pane
- **WHEN** the original connected publisher still proves its complete terminal incarnation and independent receipts place its thread in this pane and the conflicting legacy thread in a different pane, with no competing frontend owner
- **THEN** under the publication lease it preserves the exact conflicting index as private history and restores its own index without restarting the session or replaying input

#### Scenario: Conflict evidence or preservation is unavailable
- **WHEN** the conflicting index has a managed owner, the evidence is ambiguous, or the original bytes cannot be safely preserved
- **THEN** recovery leaves the conflicting index in place and reports a typed heartbeat failure
