# gtmux-tunnel-worker

The **control plane** at `api.gtmux.ccy.dev` for hosted remote access. It provisions
Standard Cloudflare named tunnels and issues per-Mac accounts for paid Direct
servers. It is separate from the [push relay](../relay-worker/).

For Standard, Cloudflare carries the tunnel traffic; this Worker drives its API.
With the checked-in `ZONE_NAME=ccy.dev`, a Mac gets a stable
`https://gtmux-<id>.ccy.dev` address. Reusing an existing registration preserves the
address across normal restarts. Replacing a deleted tunnel can change it.

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

A daily cron checks gtmux tunnel connection timestamps and removes qualifying
tunnels and their DNS. The configured thresholds are 24 hours for never-connected
tunnels and 90 days since the last active connection. This sweep does not delete
the `TUNNELS` registration records; a later provisioning call handles a confirmed
missing tunnel. Thresholds and the cron schedule are in `wrangler.toml`.
