## MODIFIED Requirements

### Requirement: Access history is bounded and owner-only
The CLI `logs --access` and owner-only `GET /api/access-history` SHALL expose only access metadata, newest first, in a paginated object. A read SHALL scan at most 4 MiB and return at most 200 entries. A continuation MAY be returned with an empty page. Cursors SHALL validate a stored basename and offset; invalid/expired cursors SHALL fail explicitly. Existing log retention SHALL apply; old activity SHALL NOT be invented.
Exact device/actor, operation and outcome filters SHALL apply before pagination. Optional device grouping SHALL count recorded activity under the same scan budget, keep full IDs distinct and use actor fallback without guessing physical identity. Per-page counters SHALL expose first/last matching time and latest metadata; grouping SHALL not retain unbounded raw entries. Cursors SHALL bind identity/operation/outcome/view, and query echoes SHALL let clients detect ignored filters.

#### Scenario: Old diagnostics precede an access record
- **WHEN** one scan reaches its byte budget before finding access metadata
- **THEN** it returns a continuation that can reach the earlier record without duplication
- **AND** guests cannot read any history, and a removed cursor yields HTTP 410

#### Scenario: Device summary across bounded pages
- **WHEN** matching records span pages or share a display name
- **THEN** each full ID remains separate, disjoint-page counts can be accumulated, and latest metadata is retained
- **AND** changing cursor filter context fails explicitly instead of skipping matches
- **AND** the counts are recorded sampled activity rather than complete request volume
