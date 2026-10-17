## MODIFIED Requirements

### Requirement: Local owners can inspect access history
Preferences SHALL offer an Access history window near paired-device management. The bounded resizable window SHALL consume the CLI access projection, support refresh and earlier pages, and keep loaded records after a read failure. Names/platform SHALL remain descriptive and HTTP acceptance SHALL NOT claim agent completion. Records SHALL use compact date-grouped log rows with inline identity/platform/IP/method/duration metadata, selectable text and Copy record, without per-record disclosures. Local 24-hour times SHALL include seconds; day headings SHALL follow the selected UI language, independently of the system locale.
The window SHALL offer Logs/Devices and exact device-ID, operation and outcome filters using the core reader. Device summaries SHALL open matching logs while preserving other filters. Incomplete summaries SHALL be labelled and manually continuable; old-query responses SHALL not overwrite current results.

#### Scenario: Local history cannot be read
- **WHEN** the CLI read fails
- **THEN** the window offers retry without presenting the failure as empty history

#### Scenario: Scanning compact access records in Chinese
- **WHEN** the owner opens access history with the Chinese UI on an English-language system
- **THEN** dates are Chinese, each local time includes seconds, and same-day rows share a date heading
- **AND** full device identity, target, metadata and HTTP outcome are readable without expanding a row
- **AND** pagination preserves source order and does not repeat the same-day heading

#### Scenario: Changing a device filter
- **WHEN** a slow older query completes after a new filter
- **THEN** only the new query is shown and unsupported filtering is not presented as success
