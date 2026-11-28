# Design: Edge and Self-Host Workers Hardening

## Context

`tunnel-worker`, `relay-worker`, `download-worker`, and the self-host `relay/` Go reference sit on public internet edges. All existing client surfaces (`gtmux` CLI, macOS menu bar, iOS/iPad/Android apps, `gtmux-authsync`, and `install.sh`) rely on stable HTTP contracts, while Cloudflare KV's eventual consistency and 1-write/sec per-key ceiling make uncoordinated read-modify-write state unsafe under concurrent access.

## Key Decisions

### 1. `DirectRegistry` SQLite Durable Object (`tunnel-worker`)
- Introduce `DirectRegistry` (`new_sqlite_classes = ["DirectRegistry"]`, migration tag `v2`, binding `DIRECT_REGISTRY`), addressed as a single global coordinator (`DIRECT_REGISTRY.getByName("registry:v1")`) with an explicit `serial()` promise queue covering network awaits (`DIRECT_CODES` code/server reads).
- Hydrate lazily once on first access from `DIRECT_CODES.get("registry:v1")` into SQLite tables (`direct_meta` + `direct_accounts`), preserving `present = false` when no legacy registry exists so `GET /direct/authfile` continues omitting `X-Gtmux-Authfile` on uninitialized deployments.
- Coalesce best-effort shadow writes back to `DIRECT_CODES.put("registry:v1", ...)` via Durable Object alarms spaced by at least 1.1s so KV `429` rate limits never fail or roll back committed Direct mutations.
- Expose `POST /direct/revoke` authenticated in constant time by `DIRECT_ADMIN_TOKEN` (falling back to `DIRECT_SYNC_TOKEN` when `DIRECT_ADMIN_TOKEN` is unset; per-server `sync` tokens are never accepted), removing minted accounts inside `DirectRegistry` before deleting the code key from `DIRECT_CODES`.

### 2. Secret-gated `/direct/servers`, IPv6 `/64` caps, and bounded reaping (`tunnel-worker`)
- `POST /direct/servers` accepts optional `secret?: string`. `current` and `acct.code`-reserved servers are populated from `deviceId` only when `secret` matches `${acct.user}:${acct.pass}` via `sameSecret`. Callers omitting `secret` (including existing `gtmux tunnel --servers` CLI builds) or sending a mismatched secret still receive the public non-reserved server list (`200`) with zero device-existence oracle.
- Normalize IPv6 `CF-Connecting-IP` values to canonical `/64` prefixes (`xxxx:xxxx:xxxx:xxxx::/64`, unwrapping IPv4-mapped `::ffff:a.b.c.d` to IPv4) before reading/writing `cap:ip:*`.
- In `reapDevice`, return `false` immediately when `rec.reaped && !rec.pending`.
- In `entry.ts` `sweep()`, iterate pages within a 20s wall-clock budget, persist `reap:cursor` only for completed pages so unvisited keys are never skipped, and run `reconcileActiveCount(env)` in `finally`.

### 3. Push relay hardening and Worker/Go twin parity (`relay-worker`, `relay/`)
- Validate APNs `token` against `/^[0-9a-fA-F]{32,512}$/` (`400 {"error":"bad token"}`) before JWT signing or `/3/device/<token>` URL construction in both `relay-worker` and `relay/`. Update shared synthetic test token fixtures in `api/testdata/apns-contract.json` to valid 64-char hex.
- Compare bearer headers in constant time over SHA-256 digests (`crypto.subtle.timingSafeEqual` on Workers with byte-loop fallback on Node test runners; `subtle.ConstantTimeCompare` in Go) so secret length and prefix timing never leak.
- Pass `signal: AbortSignal.timeout(8000)` to both Live Activity and alert/silent APNs `fetch()` calls in `relay-worker` and return `502` on timeout/transport failure.
- Add per-isolate sliding-window rate limiting keyed by normalized client IP (`/64` for IPv6) in `relay-worker` (`RELAY_IP_RPM` default `120`/min, bounded map eviction) after bearer verification, returning `429`.
- Align `relay/` with `relay-worker`: route both `POST /` and `POST /push`, return `503` when Android FCM is unconfigured, and classify client-side FCM intent errors (`liveActivity`, `>4096B` message, invalid token) as `400` in both `relay-worker/src/fcm.ts` and `relay/fcm.go` + `relay/relay.go`.

### 4. Atomic single-step downloads, `CryptoKey` memoization, and edge cache (`download-worker`)
- Cache `crypto.subtle.importKey('spki', ...)` promises per `publicKey` in a bounded module-scope `Map`, evicting rejected imports so transient errors cannot stick.
- For standard `GET` requests without a `Range` header, call `env.RELEASES.get(key)` directly once, evaluate `If-Match` / `If-None-Match` / `If-Modified-Since` / `If-Unmodified-Since` against the returned `R2Object` metadata (cancelling `object.body` on `304`/`412`), and stream `200`. Retain `head()` for `HEAD` and `Range` requests (with a single retry if a ranged object rotates between `head` and `get`).
- Consult `caches.default` (when present in the runtime) only for unconditional `GET` requests on immutable versioned paths (`!alias && key !== 'gtmux/stable.json'`), caching only `200` responses so `stable.json` promotions remain immediately readable across both download hosts.
