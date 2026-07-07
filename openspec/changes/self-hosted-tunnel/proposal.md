## Why

`gtmux tunnel` hardcodes Cloudflare (`cloudflared` → the fixed, well-known edge
`*.argotunnel.com`). Hostile networks blanket-hijack that edge domain to a dead-end
proxy, so the tunnel can't register on ANY protocol (http2/QUIC both hit the
hijacked edge) — verified live on an office network (2026-07-07). A tunnel over the
user's OWN domain/VPS on 443 is indistinguishable from normal HTTPS and can't be
blocked without breaking all TLS. The tunnel is also gtmux's intended paid tier, so
making it work on hostile networks is both the fix and the premium differentiator.

## What Changes

- Make the anywhere-tunnel **provider pluggable**: `cloudflare` (default, unchanged)
  or `self` (a self-hosted backend the user runs on their own VPS + domain).
- Add a **self-hosted backend** using a WebSocket-over-443 reverse tunnel (Chisel):
  the Mac dials out to the user's `wss://<their-domain>` server; config is manual
  (server URL + shared secret via env/config, since it's self-hosted).
- **P1 (this change's core):** the provider abstraction + `gtmux tunnel --backend self`
  (manual selection, shell out to a `chisel` client, offer to install it when
  missing — mirrors the cloudflared flow). NO automatic failover yet.
- **P2 (follow-up, spec'd but deferred here):** the pairing QR carries a PRIMARY +
  FALLBACK URL; the phone tries Cloudflare then self-hosted; auto-failover is
  triggered by the existing edge-blocked detection (PR #303's `tunnelEdgeBlocked`).
- **P3 (follow-up):** paywall gating, health monitoring, optionally embed the Chisel
  client to drop the external binary.
- Backward compatible: default stays Cloudflare; existing pairings and the
  `{url, token}` phone contract are unchanged in P1.

## Capabilities

### New Capabilities
<!-- none: extends the existing remote-access capability -->

### Modified Capabilities
- `remote-access`: the outbound-tunnel requirement gains a **pluggable provider**
  (cloudflare | self-hosted) and a self-hosted WebSocket-over-443 backend; the
  hosted-Cloudflare default is unchanged.

## Impact

- `internal/app/tunnel.go` — introduce a `tunnelProvider` seam (start client, ready
  regex, URL); cloudflared becomes one impl, a new chisel impl the other. `--backend`
  flag + `GTMUX_TUNNEL_BACKEND`; self-backend config `GTMUX_SELFTUNNEL_URL` /
  `GTMUX_SELFTUNNEL_SECRET`. `ensureChisel()` mirrors `ensureCloudflared()`.
- `internal/app/tunnelservice.go` — the always-on plist can run either backend.
- Docs: `docs/design/remote-access-tunnel.md` (provider section) + a short
  self-host setup guide (running the Chisel server on your VPS).
- P2/P3 touch the pairing QR schema (dual URL) + the mobile app; **out of scope for
  P1**, captured in tasks as deferred so the contract impact is known up front.
- Security: the self-hosted path routes through the user's VPS; the bearer token
  still gates every `/api/*` route (unchanged), and the VPS enters the trust path.
