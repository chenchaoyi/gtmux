## ADDED Requirements

### Requirement: Bounded inspectable transcript input

Transcript tool steps MAY provide optional operation, selected original input, input kind, recorded directory and a detail-truncation flag. Claude, Codex, Kimi and DSH SHALL preserve original selected strings rather than dumping arbitrary arguments. Input SHALL be bounded to 8 KiB, directory to 4 KiB and their combined additional retained bytes to 128 KiB per turn with UTF-8-safe prefixes. Cache accounting and existing response budgets SHALL include these fields. Command arrays SHALL retain JSON argument boundaries. No call result, duration or exit status SHALL be inferred.

#### Scenario: Oversized or unknown arguments

- **WHEN** a recorded input exceeds its field or turn limit, or only unrecognized arguments exist
- **THEN** the public step carries a marked prefix or no original input, respectively, while retaining its compatible summary
- **AND** no arbitrary argument object or tool output is exposed

### Requirement: Desktop navigation controls are not user dialogue

The shared Codex parser SHALL exclude a leading observed `external_codex_apps_open_page` envelope only when its body contains exactly one valid string/null `page_id`. Real text following the control SHALL remain a user prompt. Malformed controls and literal mentions after user prose SHALL remain visible.

#### Scenario: Desktop opens a conversation then the user speaks

- **WHEN** Desktop writes a page-open control and then a genuine user message
- **THEN** only the genuine message opens a user turn on every consumer of the shared transcript
