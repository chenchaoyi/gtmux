# gtmux-tunnel-worker

The **control plane** at `api.gtmux.ccy.dev` for hosted remote access. It provisions
Standard Cloudflare named tunnels and issues per-Mac accounts for paid Direct
servers. It is separate from the [push relay](../relay-worker/).

For Standard, Cloudflare carries the tunnel traffic; this Worker drives its API.
With the checked-in `ZONE_NAME=ccy.dev`, a Mac gets a stable
`https://gtmux-<id>.ccy.dev` address. Reusing an existing registration preserves the
address across restarts and replacement of a confirmed deleted tunnel. Updated always-on
clients recover automatically with their existing device identity and pairing.

## Standard flow

```
gtmux tunnel (Mac)                api.gtmux.ccy.dev (Worker)         Cloudflare API
  │  POST /provision {deviceId} ──────▶  create named tunnel  ───────▶  cfd_tunnel
  │                                      set ingress → localhost:8765
  │                                      create DNS gtmux-<id>.ccy.dev
  │  ◀── { url, token } ─────────────────┘
  │  cloudflared tunnel run --token <token>
  ▼
https://gtmux-<id>.ccy.dev  ──▶  Mac's gtmux serve :8765
```

The returned token is the **cloudflared connector token**, not a phone credential.
The phone enrolls with `gtmux pair` through the returned address. On the Mac,
`gtmux tunnel --service` installs the always-on launchd service; a foreground
`gtmux tunnel` invocation does not itself enable startup after reboot.

## Endpoints

| Method / path | Request and behavior |
| --- | --- |
| `GET /health` | Returns `{ ok: true }`. |
| `POST /standard/recover` | Stable-address recovery for the automatic watcher; requires `recover: true`, an existing device identity and the registration header. Older Workers return 404 without side effects. |
| `POST /provision` | Header `x-gtmux-reg: <REG_SECRET>`; body `{ deviceId, name?, force?, recover? }`; returns `{ url, hostname, token }`. Reuses an existing tunnel. `force` repairs in place; `recover` repairs first and replaces only a confirmed missing/deleted tunnel. The two flags cannot both be true. |
| `POST /direct/redeem` | Body `{ code, deviceId, server?, region? }`; validates a code and returns that Mac's account and route. |
| `POST /direct/servers` | Optional body `{ code?, deviceId? }`; returns offered servers and, when known, the device's current server. |
| `POST /direct/move` | Body `{ deviceId, secret, server }`; authenticates the existing account and moves the Mac to a permitted server. |
| `GET /direct/authfile` | A server-specific bearer sync token selects that server's chisel accounts. For server setup, see [self-tunnel](../deploy/self-tunnel/README.md). |

Sources: [`src/index.ts`](src/index.ts) for routing/provisioning and
[`src/direct.ts`](src/direct.ts) for Direct account and server selection.
There is no `DELETE /provision` endpoint.

## Deploy your own control plane

Use your own Cloudflare account, zone, Worker route and KV namespace IDs in
`wrangler.toml`; the checked-in values name the operated gtmux service. Install the
local development tools first:

```sh
npm install
npx wrangler kv namespace create TUNNELS
npx wrangler kv namespace create DIRECT_CODES
# Put the returned IDs in wrangler.toml and set your account/zone/route variables.
npx wrangler secret put CF_API_TOKEN   # zone DNS:Edit + account Cloudflare Tunnel:Edit
npx wrangler secret put REG_SECRET     # soft registration gate shared with your CLI
npm run deploy
```

For Direct, configure its server URL and sync credentials too; follow the
[self-tunnel server setup](../deploy/self-tunnel/README.md). A CLI using your
control plane needs `GTMUX_TUNNEL_API` (your URL) and `GTMUX_TUNNEL_REG` (your gate
value). Also set `GTMUX_TUNNEL_API_FALLBACK` to your own fallback, or to the same URL
as the primary to omit a second endpoint. Changing only the primary leaves the
compiled hosted fallback configured; setting the fallback environment value empty
restores that default.
`LOCAL_SERVICE` in `wrangler.toml` sets Standard's local forwarding target
(default `http://localhost:8765`).

## Existing abuse controls

The registration secret ships in official client builds and is only a soft gate.
New Standard tunnel creation is capped per client IP and globally. Reusing an
existing tunnel does not consume a creation slot. The checked-in limits are five
new tunnels per IP per 24-hour counter and 500 active tunnels; these KV counters
are best-effort, not atomic quotas.

The daily cron traverses registered device identities through their Durable Objects.
Online and uncertain instances are retained. Explicitly never-connected instances
become eligible after 24 hours; previously connected instances after 90 days since
`conns_inactive_at`. Deletion never forces live connections closed, and removes DNS
only after successful instance deletion and only when its target matches exactly.
The device/address reservation is retained; recovery uses the original hostname.

`src/entry.ts` is the deployed entry point. One SQLite Durable Object per device
serializes recovery and reclamation and journals partially completed operations.
Legacy KV records import on first access, including records whose instances were
removed by the previous reaper bug. KV becomes a discovery shadow; alarms coalesce
and retry shadow updates. The cron scans at most 100 keys per invocation, resuming
at a persisted cursor. Creation counters remain approximate; a bounded read-only account scan reconciles
global drift. Failed counter writes cannot strand a journaled replacement. Existing
device recovery skips first-enrollment per-IP quotas.

Always-on Macs running the updated CLI automatically request recovery after the
connection grace interval, with single-flight and capped backoff. They reload only
the connector; the original URL, serve credentials and phone permissions stay intact.
Off/Local/Direct choices take precedence over an in-flight recovery. Manual recovery:
`gtmux tunnel --backend cloudflare --service --recover --yes`.

Read the [lifecycle design](../docs/design/standard-tunnel-lifecycle.md) before changing
these rules or rolling back. Deploy the control plane, including its DO migration,
before releasing the watcher; retain the DO class and journal in any rollback.

Validation: `npm run typecheck` and `npm test` (Node unit suite plus Cloudflare
Vitest/workerd integration tests). `wrangler deploy --dry-run` checks bundling and
bindings without deploying or altering production resources.
