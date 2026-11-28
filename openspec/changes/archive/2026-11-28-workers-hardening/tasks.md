## 1. Tunnel Control Plane Hardening (`tunnel-worker`)

- [x] 1.1 Add SQLite Durable Object `DirectRegistry` (`v2` migration + `DIRECT_REGISTRY` binding) with lazy `registry:v1` hydration, alarm-coalesced KV shadow writes, and coordinated `/direct/*` routing including bearer-authenticated `POST /direct/revoke` and `revoke-direct-code.sh` support
- [x] 1.2 Gate `POST /direct/servers` `current` and code-reserved server disclosure on matching device `secret` via `sameSecret` while keeping public server listing backward compatible
- [x] 1.3 Normalize IPv6 client addresses to `/64` prefixes for `cap:ip:*`, fast-skip non-pending tombstones in `reapDevice`, and bound `entry.ts` `sweep()` wall-clock execution before `reconcileActiveCount`

## 2. Push Relay Security & Twin Parity (`relay-worker` & `relay`)

- [x] 2.1 Validate APNs tokens against `/^[0-9a-fA-F]{32,512}$/` (`400`), add `AbortSignal.timeout(8000)` (`502`) to Worker APNs fetches, enforce SHA-256 constant-time bearer comparison, and add per-IP (`/64` IPv6) sliding-window rate limiting (`RELAY_IP_RPM`) in `relay-worker`
- [x] 2.2 Align `relay/` and `relay-worker/` parity on `POST /` alongside `POST /push`, `503` for unconfigured Android FCM, `400` for client FCM validation errors, shared golden fixtures, and bilingual `relay/README*.md` docs

## 3. Release Download Worker Optimization (`download-worker`)

- [x] 3.1 Memoize imported RSA-3072 `CryptoKey` per `publicKey`, serve standard `GET` requests with one atomic `RELEASES.get`, cache immutable versioned `200` `GET`s at the edge, and rebuild `dist/`

## 4. Verification & Test Design Catalogue

- [x] 4.1 Add unit and runtime tests across `tunnel-worker`, `relay-worker`, `relay/`, and `download-worker`, map `WKH-001`..`WKH-005` in `docs/test/implementations.json`, refresh the test catalogue, and pass all worker, Go relay, design, and PR-history checks
