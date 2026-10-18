## ADDED Requirements

### Requirement: DeepSeek Harness external conversation support

The system SHALL discover and read root user conversations from the configured DSH home using a dedicated bounded v4 JSONL/Zstandard adapter. It SHALL expose public append history through the shared CLI, server, menu-bar, phone, iPad and Web readers with the official DeepSeek mark and neutral fallback. It SHALL exclude injected context, reasoning, replay metadata and raw tool results, report omitted turns, and refuse unknown generations, corrupt committed records, ambiguous identities or encodings. Readers SHALL NOT mutate DSH artifacts, acquire writer leases, invoke a model or infer live frontend ownership from saved metadata. Registry external-reader declarations SHALL have conformance-checked adapters. TUI control, adoption and usage figures SHALL NOT be implied.

#### Scenario: Existing DSH Web is already running

- **WHEN** a root user conversation exists in the configured DSH home
- **THEN** it is discoverable and readable without restarting the Web server or installing hooks
- **AND** its current controlling client remains unconfirmed and it has no pane input controls

#### Scenario: Scope and public content boundaries

- **WHEN** a reader receives a foreign home, a same-ID copy or an unsupported highest generation
- **THEN** it refuses the read rather than selecting another artifact
- **AND** a guest cannot invoke an external reader or policy mutation

### Requirement: DeepSeek Harness native events preserve explicit consent

The system SHALL provide an owned Cordis overlay and native plugin using DSH session and question APIs rather than Codex compatibility hooks. It SHALL observe only root live events, preserve successful/interrupted/failed outcomes, delegate questions unchanged, avoid treating automatic approval requests as human waits, use bounded subprocess execution and never claim its Web launcher's tmux pane. Installing SHALL preserve other profiles/plugins and SHALL NOT restart running DSH. HQ, notification and knowledge grants SHALL remain separate, revision-checked and agent/home/session scoped. Knowledge SHALL include only activity after its current grant boundary; revocation SHALL stop collection. Metadata polling SHALL NOT replay old events into HQ.

#### Scenario: Native plugin is installed for a later launch

- **WHEN** the owner installs DSH hooks
- **THEN** only two owned files are written and the next-launch overlay command is printed
- **AND** lifecycle callbacks use the dedicated native plugin without changing approval decisions

#### Scenario: Consent is revoked or a data home changes

- **WHEN** HQ follow is disabled or the same ID is copied to another home
- **THEN** dependent permissions are disabled or absent respectively
- **AND** old activity cannot become new knowledge after a later grant
