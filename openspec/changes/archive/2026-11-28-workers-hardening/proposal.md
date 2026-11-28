# Proposal: Edge and Self-Host Workers Hardening

## Why

The hosted control-plane, push relay, self-host Go relay reference, and public release download Workers share security and consistency boundaries that are hit concurrently across Cloudflare PoPs:
1. `tunnel-worker` read-modify-writes Direct accounts in KV (`registry:v1`) without cross-PoP coordination, risking lost redemptions/moves and 1-write/sec KV `429` errors, while `POST /direct/servers` discloses `current` and code-reserved servers from `deviceId` alone without verifying the device's Direct secret. Standard tunnel per-IP creation caps key full IPv6 addresses rather than `/64` prefixes, `reapDevice` still queries Cloudflare for already-reaped tombstones, and long cron sweeps have no wall-clock stop before `reconcileActiveCount`.
2. `relay-worker` and the self-host `relay/` reference interpolate APNs tokens into `/3/device/<token>` without hex validation, compare bearer secrets with plain inequality, omit an explicit timeout on Worker APNs `fetch()` calls, lack per-IP abuse throttling on `relay-worker`, and diverge on `POST /`, unconfigured Android (`503` vs `400`), and client-side FCM validation errors (`400` vs `502`).
3. `download-worker` re-imports the RSA-3072 SPKI public key on every manifest verification and performs `head()` followed by `get(..., {onlyIf: {etagMatches}})` on every standard `GET`, doubling R2 Class B operations and causing spurious TOCTOU `503` failures when `/gtmux/stable.json` advances concurrently.

## What Changes

- **Coordinated `DirectRegistry` & `/direct/servers` auth (`tunnel-worker`)**: Add SQLite Durable Object `DirectRegistry` (`v2` migration) with one-time lazy hydration from `DIRECT_CODES` `registry:v1` and alarm-coalesced shadow sync; route `/direct/redeem`, `/direct/move`, `/direct/servers`, `/direct/authfile`, and a new bearer-authenticated `POST /direct/revoke` through it; require matching `secret` on `POST /direct/servers` before disclosing `current` or `acct.code`-reserved servers; update `revoke-direct-code.sh` to use `/direct/revoke` when credentials are provided while preserving `--local` KV rehearsals.
- **Standard IPv6 `/64` quota & reaper hardening (`tunnel-worker`)**: Normalize IPv6 `CF-Connecting-IP` addresses to `/64` prefixes for `cap:ip:*`, fast-skip non-pending reaped tombstones in `reapDevice`, and bound `sweep()` to a wall-clock budget so `reconcileActiveCount` always executes.
- **Relay security hardening & Worker/Go parity (`relay-worker`, `relay`)**: Validate APNs tokens against `/^[0-9a-fA-F]{32,512}$/` (`400`), add `AbortSignal.timeout(8000)` with `502` handling to Worker APNs calls, compare bearer tokens in constant time over SHA-256 digests, enforce per-isolate sliding-window per-IP (`/64` IPv6) rate limiting (`RELAY_IP_RPM`, default 120/min) in `relay-worker`, accept `POST /` alongside `POST /push` in `relay/`, return `503` for unconfigured Android FCM in `relay/`, and return `400` for client FCM validation errors in both implementations.
- **Single-step atomic downloads & `CryptoKey` memoization (`download-worker`)**: Memoize imported RSA-3072 `CryptoKey` instances per `publicKey`, serve standard `GET` requests with one atomic `RELEASES.get(key)` while retaining `head()` for `HEAD` and `Range` requests, and cache immutable versioned `200` `GET` responses via the Worker edge cache where available without caching mutable `stable.json`.

## Surfaces

- **terminal**: `gtmux tunnel` (`--redeem`, `--servers`, `--server`, Standard provisioning), `gtmux serve` push relay delivery, `gtmux update` / `install.sh` signed release downloads, and operator `revoke-direct-code.sh` benefit transparently with backward-compatible wire contracts.
- **menubar**: Menu-bar Direct server listing/switching, Standard connection recovery, and signed release update checks keep existing contracts; unauthenticated `/direct/servers` calls continue receiving public servers while local config tracks the active Direct server.
- **phone**: iOS and Android push delivery via `relay-worker` / self-host `relay` keeps identical APNs and FCM wire payloads while rejecting malformed tokens and client validation errors cleanly.
- **iPad**: Same push notification and Live Activity delivery contract as phone; no client UI change required.
- **Web**: Browser dashboard sessions over Standard or Direct tunnels are unaffected; no Web UI change applicable because all changes are edge/self-host worker hardening.
