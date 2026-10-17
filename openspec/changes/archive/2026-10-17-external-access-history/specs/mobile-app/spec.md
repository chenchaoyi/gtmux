## MODIFIED Requirements

### Requirement: Owners can inspect server access history
Phone and iPad SHALL share a read-only access-history screen under server management, using the owner-only API and bounded pagination. It SHALL show device, time, operation, optional pane/session and HTTP outcome in compact date-grouped log rows, with full device identity and metadata inline without a disclosure. Local 24-hour times SHALL include seconds; day headings SHALL follow the selected UI language. It SHALL support refresh/earlier records and distinguish unsupported servers, denied access, expired pages and transient failures. Failed pagination SHALL preserve loaded records; server switching SHALL invalidate late responses. Guest credentials SHALL NOT request history.
Phone/iPad SHALL share Logs/Sources, exact device-ID entry/selection, operation/outcome filters and clear. Summaries SHALL accumulate bounded pages, preserve metadata and drill into exact-ID logs. Old-server filter mismatch SHALL require an update; server switching SHALL clear filters/identities and discard stale responses. Partial summaries SHALL not claim full totals.

#### Scenario: A history page expires during inspection
- **WHEN** an earlier-page request returns HTTP 410
- **THEN** already loaded records remain visible and the user can refresh from the newest page
- **AND** a response from a previously selected server cannot replace the current history

#### Scenario: Scanning compact access records in Chinese
- **WHEN** the owner opens access history with the Chinese UI on an English-language system
- **THEN** dates are Chinese, each local time includes seconds, and same-day rows share a date heading
- **AND** full device identity, target, metadata and HTTP outcome are readable without expanding a row
- **AND** pagination preserves source order and does not repeat the same-day heading

#### Scenario: Drilling into a device summary
- **WHEN** the owner selects one of two same-name devices
- **THEN** its full pairing ID selects its server-side log and other filters remain
- **AND** changing servers cannot carry that old ID or its late results into the new server

#### Scenario: External-only history and recording rules on phone and iPad
- **WHEN** either shell requests history without custom filters
- **THEN** it explicitly requests external origin and requires the matching echo and external-only rows/groups
- **AND** an older server cannot silently display mixed internal traffic as external access
- **AND** a labelled accessible Recording rules button opens coverage, sampling, retention and request-result explanations
