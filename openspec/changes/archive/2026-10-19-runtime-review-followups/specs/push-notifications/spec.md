## ADDED Requirements

### Requirement: Verify APNs wire parity across relay implementations

The Go reference relay and deployed Worker SHALL share executable contract fixtures for APNs payloads and headers, including alert, silent, waiting options and Live Activity update/end requests. The Go relay SHALL preserve the APNs rejection status in its structured error response so the real serve relay client can retire a 410 token. Live Activity SHALL use its dedicated topic and push type rather than an ordinary alert. Per-token environment SHALL select the official production or sandbox APNs endpoint; injected test gateways MAY retain their explicit endpoint.

#### Scenario: APNs reports a dead token through the Go relay

- **WHEN** APNs responds with 410
- **THEN** the relay returns a structured APNs error and the real HTTP relay client classifies the token as gone

#### Scenario: A Live Activity is updated or ended

- **WHEN** the relay receives Live Activity content state and event fields
- **THEN** both implementations emit the same dedicated APNs headers and aps fields for the shared fixture
