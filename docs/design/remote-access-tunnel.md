# Remote access from anywhere — tunnel design (2026-06-22)

How the phone reaches your Mac's agent radar **from any network, with no VPN app**.
This is the authoritative record of the "A1 hosted tunnel" architecture and the
decisions behind it. Read this before changing `gtmux tunnel`, the
`tunnel-worker/` Worker, or the remote-access docs.

## The problem

`gtmux serve` exposes a read-only radar (HTTP+SSE) on the Mac, token-gated. To use
it from the phone you need network reachability to the Mac. Three regimes:

- **Same Wi-Fi** — pair to the LAN IP. Zero setup, but only at home/office.
- **Mesh VPN (Tailscale)** — works anywhere, strongest security (E2E, nothing
  public). But it needs a VPN app on the phone, and **Tailscale is generally not
  on the mainland-China App Store** — so it can't be the default for our users.
- **From anywhere, no VPN app** — the gap this design fills.

Constraints that shaped the choice:

- **Must work for *all* users**, not just the maintainer — so "bring your own
  domain / VPS" is out as the default; it needs hosted infrastructure.
- **Must not affect the iOS app's App Store availability** (worldwide + China).
- **China reachability** matters (the maintainer's users).

## Why "outbound reverse tunnel" (not inbound, not a self-built relay)

The Mac dials **out** to a rendezvous point; the phone reaches that rendezvous via
a public URL; the rendezvous bridges the two. Outbound means **no inbound port, no
public IP, NAT solved for free**. Cloudflare runs the rendezvous (its tunnel edge)
**for free, globally** — so we do **not** build or host a data relay. We only run a
tiny *control plane* that asks Cloudflare to create a tunnel.

Rejected alternatives:

- **Self-built data relay** — heaviest: a stateful bridge we run + pay bandwidth
  for + that sees all traffic. Cloudflare already does the data plane for free.
- **Quick tunnel as the default** (`trycloudflare.com`) — zero infra, but the URL
  **rotates every run**, so the phone must re-pair constantly, and re-pairing
  needs physical access to the Mac — which defeats "phone away from the Mac". Kept
  as `--quick` for casual/testing use.
- **Per-user own domain (named tunnel)** — great for power users, impossible for
  normal users (CF account + domain + DNS). This is what hosting solves.

## Architecture (A1: hosted named tunnels)

```
gtmux tunnel (Mac)            api.gtmux.ccy.dev (Worker)          Cloudflare API
  │ POST /provision {deviceId} ─────▶ create cfd_tunnel ────────────▶ tunnel
  │   header x-gtmux-reg               set ingress → localhost:8765
  │                                    create DNS gtmux-<id>.ccy.dev
  │ ◀── { url, token } ────────────────┘
  │ cloudflared tunnel run --token <token>     (outbound, QUIC)
  ▼
https://gtmux-<id>.ccy.dev ─CF edge─▶ tunnel ─▶ Mac's gtmux serve :8765
                                                 ▲ phone pairs to this URL, ONCE
```

Two planes, two trust boundaries:

- **Control plane** — `tunnel-worker/`, a Cloudflare Worker at `api.gtmux.ccy.dev`.
  The only piece gtmux operates. `POST /provision` idempotently (keyed by a
  per-Mac `deviceId`) creates a Cloudflare **named** tunnel, points its ingress at
  `localhost:8765`, creates the DNS route, and returns the connector token. KV
  (`TUNNELS`) maps `deviceId → {tunnelId, hostname}` so re-runs reuse the tunnel.
- **Data plane** — Cloudflare's tunnel edge. The Mac's `cloudflared` connects out
  to it; the phone reaches `https://gtmux-<id>.ccy.dev`. gtmux never touches this.

The **iOS app is unchanged** — it still pairs to a `{url, token}` payload. The
transport (LAN / Tailscale / tunnel) is invisible to it, so this design has **zero
App Store impact**.

## Stable address = pair once (the whole point)

The hosted hostname is **stable** per Mac (`deviceId` is persisted at
`~/.config/gtmux/tunnel-device-id`; provisioning is idempotent). So the phone
pairs **once** and keeps working across `gtmux tunnel` restarts and Mac reboots —
unlike a quick tunnel whose URL rotates. "Always-on across reboot" additionally
needs a launchd service (see *Not yet built*).

## Naming + the single-level TLS constraint (important)

User tunnels are **single-level**: `gtmux-<id>.ccy.dev`, **not** `<id>.gtmux.ccy.dev`.

Cloudflare's free **Universal SSL covers only one subdomain level** (`ccy.dev` and
`*.ccy.dev`). A 3rd-level host like `<id>.gtmux.ccy.dev` gets **no edge cert** →
TLS handshake failure. A wildcard for `*.gtmux.ccy.dev` would need paid Advanced
Certificate Manager. So we keep tunnels at a single level and namespace them with
the `gtmux-` **prefix** instead of a `gtmux.` label. The control-plane Worker
(`api.gtmux.ccy.dev`) is a Workers **custom domain**, which provisions its own
dedicated cert regardless of depth (takes a few minutes on first deploy).

## Security model

- **Two independent token layers:**
  - **Connector token** (cloudflared ↔ CF) — authorizes this Mac as the tunnel's
    connector. Returned by `/provision`, run via `cloudflared tunnel run --token`.
  - **Serve bearer token** (phone ↔ serve, end-to-end through the tunnel) — the
    existing `~/.config/gtmux/serve-token`. Every `/api/*` route checks it
    (no token → 401), **unchanged over a public URL**.
- **A public URL makes the bearer token the sole gate** to the read-only radar —
  there's no VPN layer in front anymore. The API stays read-only (no `send-keys`,
  no input injection), but treat URL + token like a password; don't screenshot the
  pairing QR into shared channels. The CLI says this in its output.
- **`x-gtmux-reg` soft gate** — a registration secret the CLI sends to
  `/provision`. It necessarily ships in the binary (injected at release build from
  the `GTMUX_TUNNEL_REG` CI secret), so it is **not** a real secret — just a speed
  bump against casual abuse of the endpoint. Real protection is the hardening
  below.
- **Privacy** — Cloudflare terminates TLS at its edge, so it can see plaintext
  radar traffic (as with any CF tunnel). Acceptable for read-only pane metadata;
  app-layer E2E encryption is a possible future increment.

## What gtmux operates (ownership + cost)

- The `ccy.dev` zone on Cloudflare + the `gtmux-tunnel` Worker + the `TUNNELS` KV.
- Secrets in the Worker: `CF_API_TOKEN` (scoped: `ccy.dev` DNS:Edit + account
  Cloudflare Tunnel:Edit) and `REG_SECRET`.
- Cost ≈ the domain; Workers + KV + Cloudflare Tunnel are within free tiers at this
  scale. Cloudflare carries the bandwidth (no egress fees).
- **Centralization risk** — if this infra stops, hosted remote access stops. So
  the **bring-your-own paths stay supported** (Tailscale; `--quick`; and
  `GTMUX_TUNNEL_API` / `GTMUX_TUNNEL_REG` overrides let self-hosters point at their
  own Worker).

## Self-hosting

`gtmux tunnel` reads `GTMUX_TUNNEL_API` and `GTMUX_TUNNEL_REG` at runtime
(overriding the build-time defaults). Deploy your own `tunnel-worker/` to your own
zone, set those two, and the CLI uses your control plane instead of gtmux's.

## Testing caveat — corporate DNS interception

On networks that do **transparent DNS interception + per-domain categorization**
(e.g. the maintainer's office, which rewrites even `8.8.8.8`/`1.1.1.1` answers to
internal `172.19.2.x` proxy IPs), **brand-new `ccy.dev` hostnames are mangled**
until the proxy categorizes them, so the final "public hostname → tunnel" hop
**can't be curl-verified from that network**. The control plane (provision) and the
Mac→CF half (cloudflared registers) are verifiable there; the last hop is verified
from the **phone on cellular/home** (a normal network hits real CF IPs). This is a
network artifact, not a design flaw, and does not affect real users.

## Always-on (explicit opt-in)

By default `gtmux tunnel` runs in the **foreground** — you consciously open remote
access for a session; Ctrl-C stops it. The stable URL already means no re-pairing
across manual restarts. **Always-on** (reachable across reboots without re-running)
is a separate, opt-in, reversible mode — never a default, because a standing
public exposure should be a conscious choice and stay visible:

- `gtmux tunnel --service` — provisions the stable tunnel, then registers two
  per-user **LaunchAgents** (`com.gtmux.serve` → `gtmux serve` on loopback;
  `com.gtmux.tunnel` → `cloudflared` with the connector token), `RunAtLoad` +
  `KeepAlive`. It explains the standing exposure and asks first (`--yes` bypasses
  the prompt — used by the menu-bar toggle, which shows its own confirmation).
- `gtmux tunnel --unservice` — unloads + deletes both agents.
- `gtmux tunnel --status` — on/off + the stable URL.
- The connector token lives in the tunnel plist (0600). The menu-bar app surfaces
  an on/off toggle + a visible indicator so always-on is never silent.

## Not yet built (tracked)

- **Abuse hardening** — per-`deviceId` cap, reap tunnels unused for N days,
  `DELETE /provision`, rate limiting. The `x-gtmux-reg` gate is only a speed bump.
- **App-layer E2E encryption** — so Cloudflare can't see radar plaintext.
- **Menu-bar "Allow phone access"** — produce the pairing QR from the app, fed by
  the tunnel address.

## Providers: Cloudflare (default) vs self-hosted (P1)

`gtmux tunnel` has a pluggable backend (`--backend cloudflare|self`, or
`GTMUX_TUNNEL_BACKEND`):

- **`cloudflare`** (default) — the zero-config hosted address above. Works on most
  networks, but a hostile network can DNS-hijack Cloudflare's edge (`*.argotunnel.com`)
  and kill it on every protocol (see the debug runbook).
- **`self`** — a WebSocket-over-443 tunnel (Chisel) to **your own VPS + domain**,
  indistinguishable from ordinary HTTPS, so it survives that hijack. You run the
  server side (chisel + a TLS reverse proxy, coexisting with anything already on 443
  via an SNI router) — see **`deploy/self-tunnel/`** for the versioned config +
  install/migration scripts. Config is manual (your own server):
  `GTMUX_SELFTUNNEL_URL` (`https://tunnel.example.com`) + `GTMUX_SELFTUNNEL_SECRET`
  (chisel `user:pass`). gtmux fetches the chisel client itself. The phone pairs to
  `{url, token}` exactly as with Cloudflare. `--service` registers it always-on.

P1 is manual selection; auto-failover Cloudflare→self and a dual-URL pairing QR are
P2 (see `openspec/changes/.../self-hosted-tunnel`). The tunnel is the intended paid tier.

## Debug runbook (pairing / reachability)

Consolidated in `docs/TROUBLESHOOTING.md`; the essentials for this subsystem:

1. **"Pairing code expired" that won't clear → duplicate serve on :8765.** The
   menubar mints via `127.0.0.1:8765` (IPv4) but the tunnel's `localhost:8765`
   resolves to `::1` (IPv6); if a second `gtmux serve` binds `*:8765`, mint and
   redeem hit different processes and enroll codes (in-memory) don't match. Check:
   `lsof -nP -iTCP:8765 -sTCP:LISTEN` must show exactly ONE PID (the app's
   `com.gtmux.serve`). Kill any bare `gtmux serve` squatting `*:8765`.
2. **Don't restart serve between mint and scan** — codes are in-memory (TTL 5 min);
   `launchctl kickstart`/`unload+load` wipes them → "expired".
3. **Tunnel offline on corp net = QUIC blocked.** `tunnel.log` loops `failed to dial
   to edge with quic`; phone gets CF 1033/530. Fix = `--protocol http2` (now the
   default; `GTMUX_TUNNEL_PROTOCOL` overrides). An old service plist keeps QUIC —
   re-run `gtmux tunnel --service` after `gtmux update`.
4. **Corp-DNS hijack** rewrites `ccy.dev` to `172.19.x` → the Mac's own probe fails
   on a healthy tunnel; verify from a **phone on cellular**.

## Code map

| Piece | Where |
|---|---|
| CLI hosted + quick modes, cloudflared runner, QR | `internal/app/tunnel.go` |
| Build-time API URL + reg gate (env-overridable) | `internal/app/tunnelconfig.go` |
| Control-plane Worker (provision via CF API) | `tunnel-worker/src/index.ts` |
| Deploy config (account/zone/KV ids, domain) | `tunnel-worker/wrangler.toml` |
| Reg-gate injection | `Makefile`, `.goreleaser.yaml`, `.github/workflows/release.yml` |
| Capability spec | `openspec/specs/remote-access/spec.md` |
