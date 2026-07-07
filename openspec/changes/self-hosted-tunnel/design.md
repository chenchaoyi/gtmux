## Context

`internal/app/tunnel.go` runs cloudflared directly: `runCloudflared(bin, args,
readyRe, onReady)` starts the process, watches its log for a ready line
(`registeredRe` / `tryCloudflareRe`), and prints the pairing block with the URL.
Three call sites (hosted `tunnel run`, quick `tunnel --url`, and the service plist
in `tunnelservice.go`) hardcode cloudflared. The phone pairs to a single
`{url, token}`. PR #303 added `tunnelEdgeBlocked()` (reads the tunnel log) — a ready
trigger for auto-failover later.

## Goals / Non-Goals

**Goals (P1):**
- A `tunnelProvider` seam so the anywhere tunnel can be Cloudflare (default) or a
  self-hosted WebSocket-over-443 backend, selected explicitly.
- Get the user unblocked on a hostile network by dialing their own VPS over 443.

**Non-Goals (P1 — deferred to P2/P3, spec'd but not built here):**
- Automatic failover Cloudflare→self. Dual-URL pairing QR. Mobile-app changes.
- A managed/provisioned self-hosted server (the user runs their own Chisel server).
- Paywall gating. Embedding the Chisel client.

## Decisions

- **Backend = Chisel** (WebSocket-over-443, single Go binary). Shell out to a
  `chisel client` like we shell out to cloudflared (consistent; `ensureChisel()`
  offers `brew install chisel`). Embedding the client (Chisel is Go) is a P3
  optimization to drop the external binary — noted, not done now.
- **Provider seam**: a small interface — `name`, `command(port) []string`,
  `readyRe *regexp.Regexp`, `urlFrom(line/config) string`. cloudflared and chisel
  each implement it; the three run paths call through it. Keeps the log-watch/ready/
  pairing-print machinery shared.
- **Selection**: `gtmux tunnel --backend cloudflare|self` (default cloudflare) and
  `GTMUX_TUNNEL_BACKEND`. Self-backend needs the user's server: `GTMUX_SELFTUNNEL_URL`
  (`wss://tunnel.example.com`) + `GTMUX_SELFTUNNEL_SECRET` (shared auth). Missing
  config → a clear "set these to use the self-hosted backend" message, not a crash.
- **Public URL**: for self-hosted, the reachable phone URL is the user's own
  `https://<their-domain>` (mapped to the tunneled local port by their Chisel server
  + reverse proxy). gtmux prints it in the pairing block exactly like the CF URL, so
  the phone pairs identically — `{url, token}` contract unchanged in P1.
- **Server side (user-run, documented, not shipped)**: a `chisel server` on the VPS
  behind their TLS-terminating proxy on 443, with the shared secret. A short setup
  guide in `docs/design/remote-access-tunnel.md`.

## Risks / Trade-offs

- **Chisel as a second external dep**: another `brew install`. Mitigated by mirroring
  the cloudflared install flow; P3 can embed it.
- **Self-host burden**: the user runs+maintains a VPS server. Acceptable — this is
  the advanced/paid path, opt-in; Cloudflare stays the zero-config default.
- **VPS in the trust path**: the tunnel now transits the user's own server. Same
  bearer-token gating on every route; the VPS sees only encrypted-to-serve traffic
  it proxies. Document that the token is still the sole app-layer gate.
- **P2 dual-URL is a contract change** (pairing QR gains a fallback URL): flagged now
  so P1's QR stays single-URL and forward-compatible (extra field, older apps ignore).
