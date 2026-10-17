## MODIFIED Requirements

### Requirement: Owner access history records verified request metadata
The server SHALL write informational access actions to the existing private diagnostic store for authenticated API operations and successful code redemption. Metadata SHALL contain time, unique ID, verified credential scope/device identity, descriptive name/platform, trusted source IP, allowlisted operation, optional pane locator or structured Codex session UUID, HTTP result and duration. It SHALL NOT contain credentials, codes, query strings, request/response bodies, command or conversation text. Master credentials SHALL NOT imply an identifiable device. Successful GET/HEAD reads SHALL be sampled once per five minutes per origin/full identity/operation/pane or session (and source IP without a verified device); read failures, ordinary mutations and HQ archive exports SHALL remain individual. Archive generation failures SHALL be recorded as failed even if HTTP headers are already committed. Creation receipts SHALL include the returned pane, and notification-registration reads SHALL have an explicit category. A router-derived test SHALL require an explicit classification or documented exception for every registered API. Stream handlers SHALL record actual opens/closes and failed opens. Unknown credentials SHALL retain bounded rejection receipts once per minute per external/local/unknown origin, independent of the global diagnostic warning sampler; static assets and health probes are excluded.

#### Scenario: A paired phone supplies a forged actor header
- **WHEN** a verified device sends terminal input with a different actor header
- **THEN** history retains its verified full device ID, pane and HTTP result, without the input text or authorization value
- **AND** two input requests remain two records while repeated successful reads are coalesced

#### Scenario: Repeated exports and separate desktop conversations
- **WHEN** one device downloads the HQ archive twice and reads two different verified desktop sessions within five minutes
- **THEN** both exports and both conversation targets are recorded separately
- **AND** an archive-generation error is failed even after HTTP 200 was committed, while unsupported methods are refused
- **AND** arbitrary query/payload text cannot become a session locator

#### Scenario: Local tools and external clients share a credential
- **WHEN** a loopback master request identifies a recognized local tool and a LAN request uses the same credential and actor
- **THEN** they are recorded as local and external respectively, with separate successful-read sampling windows
- **AND** local/unknown rejections cannot consume the external rejection receipt


### Requirement: Access history is bounded and owner-only
The CLI `logs --access` and owner-only `GET /api/access-history` SHALL expose only access metadata, newest first, in a paginated object. A read SHALL scan at most 4 MiB and return at most 200 entries. A continuation MAY be returned with an empty page. Cursors SHALL validate a stored basename and offset; invalid/expired cursors SHALL fail explicitly. Existing log retention SHALL apply; old activity SHALL NOT be invented.
Origin, exact device/actor, operation and outcome filters SHALL apply before pagination. Optional device grouping SHALL count recorded activity under the same scan budget, keep full IDs distinct and use actor fallback without guessing physical identity. Per-page counters SHALL expose first/last matching time and latest metadata; grouping SHALL not retain unbounded raw entries. Cursors SHALL bind origin/identity/operation/outcome/view, and query echoes SHALL let clients detect ignored filters.

#### Scenario: Old diagnostics precede an access record
- **WHEN** one scan reaches its byte budget before finding access metadata
- **THEN** it returns a continuation that can reach the earlier record without duplication
- **AND** guests cannot read any history, and a removed cursor yields HTTP 410

#### Scenario: Device summary across bounded pages
- **WHEN** matching records span pages or share a display name
- **THEN** each full ID remains separate, disjoint-page counts can be accumulated, and latest metadata is retained
- **AND** changing cursor filter context fails explicitly instead of skipping matches
- **AND** the counts are recorded sampled activity rather than complete request volume

#### Scenario: External history is the default projection
- **WHEN** a CLI or API client requests history without an explicit origin
- **THEN** only externally identified records participate in entries, summaries, filter choices and pagination limits
- **AND** explicit external/local/unknown/all diagnostic ranges remain available without rewriting or deleting stored records
- **AND** cursors from another origin or old unbound cursors fail explicitly

## ADDED Requirements

### Requirement: Access origin uses verified identity and conservative source evidence
Access records SHALL store external/local/unknown origin and an allowlisted evidence category. A verified paired device or shared-link identity SHALL remain external regardless of loopback/proxy transport. A valid non-loopback peer or forwarded address trusted by existing proxy rules SHALL count as external regardless of credential scope or actor. A loopback master request with a recognized tool actor and no forwarding hints MAY be classified local. Master scope or loopback alone SHALL NOT prove local or external origin. Ambiguous sources SHALL remain unknown and available in diagnostics, outside the default external view. Client-supplied descriptive headers SHALL NOT create a verified identity.
Historical records without origin SHALL be interpreted as external only from an existing verified full device ID or valid non-loopback recorded source IP; otherwise they SHALL remain unknown. Reads SHALL NOT modify stored history, invent devices, or reconstruct precise operations from missing fields. Explicit unknown origin SHALL NOT be upgraded from other fields.

#### Scenario: Paired clients traverse a headerless proxy
- **WHEN** a verified phone/browser/share client reaches serve over loopback with no usable forwarding header
- **THEN** its access, including refused and failed requests, remains external under its verified identity
- **AND** forged actor or display headers cannot turn it into a local menu-bar request

#### Scenario: Old records lack source evidence
- **WHEN** an old owner record has only a loopback address or no source fields
- **THEN** it remains unknown, does not appear as an external device, and remains readable with the diagnostic unknown/all range
