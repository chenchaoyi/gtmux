# gtmux-tunnel-worker

The **control plane** for hosted, no-VPN remote access (the "A1" architecture).
A tiny Cloudflare Worker at `api.gtmux.ccy.dev` that the `gtmux tunnel` CLI calls
to **provision a per-Mac Cloudflare named tunnel** under `*.gtmux.ccy.dev`.

Cloudflare carries the data plane (free, global); this Worker is the only piece
gtmux operates. The phone app is unchanged — it still pairs to a `{url, token}`,
now with a **stable** URL so it pairs once and keeps working across reboots.

## Flow

```
gtmux tunnel (Mac)                api.gtmux.ccy.dev (Worker)         Cloudflare API
  │  POST /provision {deviceId} ──────▶  create named tunnel  ───────▶  cfd_tunnel
  │                                      set ingress → localhost:8765
  │                                      create DNS <id>.gtmux.ccy.dev
  │  ◀── { url, token } ─────────────────┘
  │  cloudflared tunnel run --token <token>   (+ launchd, always-on)
  ▼
https://<id>.gtmux.ccy.dev  ──▶  Mac's gtmux serve :8765   (phone pairs to this, once)
```

## Endpoints

- `GET  /health` → `{ ok: true }`
- `POST /provision` — header `x-gtmux-reg: <REG_SECRET>`, body `{ deviceId, name? }`
  → `{ url, hostname, token }`. Idempotent per `deviceId` (reuses the tunnel).

## Deploy

```sh
npm install
wrangler kv namespace create TUNNELS      # paste the id into wrangler.toml
wrangler secret put CF_API_TOKEN          # zone ccy.dev DNS:Edit + account Cloudflare Tunnel:Edit
wrangler secret put REG_SECRET            # also baked into the gtmux CLI build
# set CF_ACCOUNT_ID + CF_ZONE_ID (ccy.dev) in wrangler.toml [vars]
wrangler deploy
```

## TODO before GA

- Abuse hardening: per-deviceId cap, reap tunnels unused for N days, rate limit.
- `DELETE /provision` to tear a device's tunnel down.
- Make `LOCAL_SERVICE` port configurable (default 8765).
