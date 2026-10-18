## ADDED Requirements

### Requirement: Reuse the running core radar without changing refresh cadence

The menu bar SHALL prefer an authenticated loopback snapshot of the running serve for its normal agents poll. Serve SHALL share one single-flight producer snapshot across HTTP and SSE readers for at most 1.5 seconds, retaining at most 4 MiB, without exposing mutable cached storage or crossing profile homes. The menu bar SHALL require a fresh running status file and a saved owner credential, reject redirects and oversized or malformed responses, and fall back to the existing fresh CLI reader on failure with a bounded retry interval. Explicit CLI/native-home overrides SHALL retain their standalone reader. Other CLI reads and the configured poll cadence SHALL remain unchanged.

#### Scenario: Multiple surfaces read the same radar

- **WHEN** the menu bar, HTTP and SSE readers request within the shared snapshot interval
- **THEN** they reuse the same core collection without a second menu-bar subprocess

#### Scenario: The service is stale or unusable

- **WHEN** serve status is stale, the request fails, or its response is invalid
- **THEN** the menu bar reads the standalone CLI and retries the shared service after its cooldown
