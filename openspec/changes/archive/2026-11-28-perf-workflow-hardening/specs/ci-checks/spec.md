## ADDED Requirements

### Requirement: Incremental test-design history preserves historical mappings
When a PR modifies an existing test file and appends newly reviewed case IDs in `implementations.json`, `--pr-history` SHALL verify that every newly added case ID was sealed in an earlier commit without requiring untouched historical case plans to be re-sealed. Modifying an existing test file without adding a new case ID SHALL still require at least one mapped case plan to be freshly sealed before the first test edit.

#### Scenario: Appending a reviewed case to an existing test file
- **WHEN** an existing test file retains its historical case ID and adds a new case ID sealed in an earlier PR commit
- **THEN** `--pr-history` succeeds without modifying the historical design seal
- **AND** editing the test file with neither a new case ID nor a freshly sealed existing plan fails

### Requirement: Scoped mobile release metadata and non-shared RN UI checks
Mobile release metadata (`.srchash`, `android.json`, `GtmuxMobile.xcodeproj`) SHALL select only affected mobile, iOS, and Android checks, and non-shared RN UI sources SHALL omit `web` while Web-consumed `mobileapp/` sources continue selecting `web`.

#### Scenario: Pure RN component and mobile release prep
- **WHEN** a PR edits a non-Web-shared RN UI component or mobile release-note metadata
- **THEN** `ci.py` selects the affected mobile and native suites without `menubar`, Go `test`, or `web`

### Requirement: Sharded Android phone CI and SHA-pinned Node 24 workflows
Android phone and tablet CI SHALL each execute across two verified shards under their aggregate check names. Workflow files SHALL pin third-party actions by commit SHA on Node 24 runtimes, avoid moving `macos-latest` aliases, and include `./scripts/check-design.sh --source-only` in `make check`.

#### Scenario: Android phone and tablet shard verification
- **WHEN** Android CI runs on a mobile change
- **THEN** both phone and tablet verify all 31 cases across two parallel shards under `Android phone emulator flows` and `Android tablet emulator flows`
