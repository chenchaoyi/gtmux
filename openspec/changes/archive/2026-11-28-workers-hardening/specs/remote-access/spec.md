## ADDED Requirements

### Requirement: Coordinated Direct registry and secret-gated route disclosure

The tunnel control plane SHALL serialize Direct account mutations and server authfile reads through a strongly consistent SQLite Durable Object hydrated once from legacy KV, tolerating KV shadow write rate limits. `POST /direct/servers` SHALL disclose a device's current server and code-reserved servers only when the caller presents that device's matching account secret, and `POST /direct/revoke` SHALL authenticate an admin or sync bearer.

#### Scenario: Concurrent Direct mutations preserve all accounts

- **WHEN** multiple devices simultaneously redeem, move, and sync authfiles across edge locations within one second
- **THEN** the `DirectRegistry` Durable Object serializes every mutation in SQLite without lost accounts, duplicate ports, or KV `429` failures

#### Scenario: Unauthenticated server listing omits device assignment

- **WHEN** a caller POSTs `{ deviceId }` to `/direct/servers` without that device's matching Direct `secret`
- **THEN** the control plane returns `200` with only public non-reserved servers and omits `current` and code-reserved servers
- **AND** a caller presenting both `deviceId` and its matching `secret` receives `current` and servers reserved for its code

#### Scenario: Coordinated code revocation

- **WHEN** an operator POSTs `{ code }` to `/direct/revoke` with the configured admin or sync bearer token
- **THEN** `DirectRegistry` atomically removes all accounts minted by that code before deleting the code record, and unauthenticated callers receive `401`

### Requirement: Standard tunnel IPv6 prefix quotas and bounded reaper sweeps

The tunnel control plane SHALL normalize IPv6 client addresses to their `/64` network prefix before keying per-IP Standard tunnel creation counters. Device reclamation SHALL immediately skip already-reaped tombstones that carry no pending replacement without provider API queries, and scheduled sweeps SHALL persist cursor progress within a bounded time budget before reconciling active counts.

#### Scenario: IPv6 interface rotation shares one creation cap

- **WHEN** unregistered devices request new Standard tunnels from distinct addresses within the same IPv6 `/64` prefix
- **THEN** all requests count against the same `cap:ip:<prefix>` 24-hour ceiling and return `429` once `PROVISION_IP_CAP` is reached

#### Scenario: Reaped tombstones skip provider lookups and sweeps stay bounded

- **WHEN** the scheduled sweep visits an already-reaped tombstone without a pending replacement or hits its wall-clock sweep budget
- **THEN** tombstones return immediately with zero Cloudflare Tunnel API calls, `reap:cursor` preserves unvisited keys, and `reconcileActiveCount` still runs
