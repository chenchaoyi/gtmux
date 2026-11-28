## ADDED Requirements

### Requirement: Hardened push relay validation, rate limits, and reference parity

Both push relay implementations SHALL compare bearer tokens in constant time over SHA-256 digests, accept `POST /` alongside `POST /push`, validate APNs device tokens against `/^[0-9a-fA-F]{32,512}$/` with HTTP `400` before gateway requests, return HTTP `503` when Android FCM is unconfigured, and return HTTP `400` for invalid client FCM intents. The Worker relay SHALL bound APNs fetches to 8 seconds and rate-limit per-IP (`/64` IPv6) bursts.

#### Scenario: Malformed APNs token or APNs timeout

- **WHEN** an authenticated caller sends an APNs token outside `/^[0-9a-fA-F]{32,512}$/` or APNs exceeds the 8-second timeout
- **THEN** invalid tokens fail closed with `400 {"error":"bad token"}` before gateway calls, and Worker APNs timeouts return `502`

#### Scenario: Scraped relay bearer floods from one IP or IPv6 /64

- **WHEN** authenticated push requests from one IPv4 address or IPv6 `/64` prefix exceed `RELAY_IP_RPM` within one minute
- **THEN** `relay-worker` returns `429` before contacting APNs or FCM

#### Scenario: Android FCM parity across Worker and Go relays

- **WHEN** an authenticated caller posts an Android intent to `/` or `/push`
- **THEN** both relays return `503` when FCM is unconfigured, `400` for `liveActivity`, oversized (`>4096B`), or malformed-token intents, and `502` for provider failures
