## MODIFIED Requirements

### Requirement: Owner access history records verified request metadata
The server SHALL write informational access actions to the existing private diagnostic store for authenticated API operations and successful code redemption. Metadata SHALL contain time, unique ID, verified credential scope/device identity, descriptive name/platform, trusted source IP, allowlisted operation, optional pane locator or structured Codex session UUID, HTTP result and duration. It SHALL NOT contain credentials, codes, query strings, request/response bodies, command or conversation text. Master credentials SHALL NOT imply an identifiable device. Successful GET/HEAD reads SHALL be sampled once per five minutes per full identity/operation/pane or session; read failures, ordinary mutations and HQ archive exports SHALL remain individual. Archive generation failures SHALL be recorded as failed even if HTTP headers are already committed. Creation receipts SHALL include the returned pane, and notification-registration reads SHALL have an explicit category. A router-derived test SHALL require an explicit classification or documented exception for every registered API. Stream handlers SHALL record actual opens/closes and failed opens. Unknown credentials SHALL retain the bounded global rejection sampler; static assets and health probes are excluded.

#### Scenario: A paired phone supplies a forged actor header
- **WHEN** a verified device sends terminal input with a different actor header
- **THEN** history retains its verified full device ID, pane and HTTP result, without the input text or authorization value
- **AND** two input requests remain two records while repeated successful reads are coalesced


#### Scenario: Repeated exports and separate desktop conversations
- **WHEN** one device downloads the HQ archive twice and reads two different verified desktop sessions within five minutes
- **THEN** both exports and both conversation targets are recorded separately
- **AND** an archive-generation error is failed even after HTTP 200 was committed, while unsupported methods are refused
- **AND** arbitrary query/payload text cannot become a session locator
