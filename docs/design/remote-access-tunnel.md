# Remote access from anywhere — tunnel design (2026-06-22)

This records the June 2026 choice of the A1 hosted tunnel and summarizes the
implementation as of 2026-10-06. The June rationale below is historical; the
architecture and delivered-status sections describe the current code. Read this
before changing `gtmux tunnel`, `tunnel-worker/`, or the remote-access docs.

## The original problem and goals (June 2026)

The initial radar needed a reachable Mac address without asking every user to run
a VPS or install a phone VPN. The options considered were a LAN address, a mesh
VPN such as Tailscale, and an outbound hosted tunnel. Supporting users in mainland
China and keeping the iOS app eligible for distribution were design goals, not
proof that any transport works on every network or that App Store review is
unaffected.

The current `gtmux serve` exposes both reads and authorized controls over HTTP,
SSE and WebSocket. An owner credential can send terminal input and create sessions;
it is no longer a read-only radar. LAN access requires a reachable interface and
firewall rules; a shared Wi-Fi name alone does not establish reachability.

## Why an outbound reverse tunnel (the June choice)

The Mac connects out to a rendezvous point that exposes an HTTPS URL to the phone.
This avoids opening an inbound port on the Mac or requiring a public IP there;
the Mac's outbound connection and the phone's route to the public endpoint must
still be permitted by their networks.

For the original Standard backend, Cloudflare supplies the tunnel data plane and
gtmux supplies a control plane that provisions it. A self-operated data relay was
rejected as the initial default because it added server and bandwidth operations;
the later Direct backend does use separately operated servers.

A quick tunnel (`--quick`) remains useful for temporary access, but has an
ephemeral URL that must be updated on clients when it changes. Requiring each user
to own a domain and Cloudflare account was also rejected as the default; hosted
provisioning supplies those resources for Standard.

## Architecture (A1: hosted named tunnels)

```
gtmux tunnel (Mac)            api.gtmux.ccy.dev (Worker)          Cloudflare API
  │ POST /provision {deviceId} ─────▶ create cfd_tunnel ────────────▶ tunnel
  │   header x-gtmux-reg               set ingress → localhost:8765
  │                                    create DNS gtmux-<id>.ccy.dev
  │ ◀── { url, token } ────────────────┘
  │ cloudflared tunnel run --token <token>     (outbound, http2 — QUIC is often blocked)
  ▼
https://gtmux-<id>.ccy.dev ─CF edge─▶ tunnel ─▶ Mac's gtmux serve :8765
                                                 ▲ phone pairs through this URL
```

Two planes, two trust boundaries:

- **Control plane** — `tunnel-worker/`, a Cloudflare Worker at `api.gtmux.ccy.dev`.
  `POST /provision` idempotently (keyed by a
  per-Mac `deviceId`) creates a Cloudflare **named** tunnel, points its ingress at
  `localhost:8765`, creates the DNS route, and returns the connector token. KV
  (`TUNNELS`) maps `deviceId → {tunnelId, label, hostname}` so re-runs reuse the tunnel.
- **Data plane** — Cloudflare's tunnel edge. The Mac's `cloudflared` connects out
  to it; the phone reaches `https://gtmux-<id>.ccy.dev`. The provisioner does not
  proxy this traffic. Here `<id>` is a random label, not the Mac's `deviceId`.

Current pairing normally carries `{v:2, url, enrollCode}` (and optionally a display
name), which the app redeems for its own token. Legacy v1 `{url, token}` pairing
remains supported. The same API is reached through LAN, Standard or Direct, but
that transport design alone says nothing about App Store review or privacy
classification.

## Stable address across normal restarts

The Mac's `deviceId` is persisted in `~/.config/gtmux/tunnel-device-id`. Reusing its
existing Standard registration preserves the address across normal restarts.
Replacing a confirmed deleted tunnel can change the address; a revoked device
credential also needs new pairing. Background startup is already implemented via
`--service` (see *Always-on* below); a stable URL by itself does not keep the Mac
awake, online or logged in.

## Naming and TLS coverage

User tunnels use `gtmux-<id>.ccy.dev`, not `<id>.gtmux.ccy.dev`. The checked-in
`ZONE_NAME` is `ccy.dev`; self-hosters choose their own zone.

With Cloudflare's full DNS setup, Universal SSL covers the apex and one subdomain
level. Deeper names need separate certificate coverage; partial CNAME setups have
different coverage rules. See [Cloudflare's Universal SSL limitations](https://developers.cloudflare.com/ssl/edge-certificates/universal-ssl/limitations/).
The control-plane route in `wrangler.toml` is a Workers Custom Domain, for which
Cloudflare issues a certificate, including multi-level names; see
[Custom Domains](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/).
These are configuration requirements, not a readback of a production account's
certificate status.

## Security model

- **Two independent credential layers:** the connector token returned by
  `/provision` authorizes `cloudflared` to connect the Mac to the tunnel. The
  serve bearer credential authorizes the client API request; a paired device
  normally uses its own token rather than the Mac's master token.
- **API access includes writes.** Treat owner credentials and valid owner pairing
  codes as keys to the Mac's terminal. `/api/health` is public, and `/api/enroll`
  uses its code as the credential instead of requiring a bearer token. The other
  API routes are bearer-gated and apply their own caller permissions. See the
  [API contract](../../api/contract.md) for those permissions.
- **`x-gtmux-reg` is a soft gate.** The release build injects the registration
  value into the binary. It is not a private client secret. The Worker also has
  creation caps and unused-tunnel reclamation, described below.
- **TLS is not application-layer E2E.** Cloudflare terminates Standard's TLS and
  can see session API traffic, including terminal output and submitted input.
  Direct's TLS proxy operator is likewise in the trust path. Push has a separate
  relay path. See the [security model](SECURITY.md).

## Operated components and self-hosting

Standard uses the `ccy.dev` zone, `gtmux-tunnel` Worker and `TUNNELS` KV. The
Worker needs `CF_API_TOKEN` (zone DNS:Edit and account Cloudflare Tunnel:Edit) and
`REG_SECRET`. It also has Direct provisioning routes; Direct tunnel servers and
the APNs push relay are separate operated components. The original design's
free-tier cost assumption is not a guarantee about current usage or billing.

If the hosted control plane is unavailable, provisioning and repair can fail;
an already-running tunnel's data path is separate. Cloudflare or the selected
Direct server going down can interrupt active remote access.

For your own Standard control plane, deploy [tunnel-worker](../../tunnel-worker/README.md)
with your account, zone, route and KV IDs. Configure all three CLI values:

- `GTMUX_TUNNEL_API`: your Worker's URL.
- `GTMUX_TUNNEL_REG`: its registration gate value, not a URL.
- `GTMUX_TUNNEL_API_FALLBACK`: your own fallback URL, or the same URL as the primary
  to omit the second endpoint. Changing only the primary leaves the hosted
  fallback configured; an empty environment value restores the compiled default.

A foreground shell's environment is not a launchd service configuration. For a
service, ensure its process receives the intended configuration; do not assume
variables exported only in the installing terminal persist across login.

## Network verification

DNS interception and network policy can affect both testing and real users.
A successful provision response or registered Mac-to-edge connector does not
prove the phone-to-public-hostname path works. Test that last hop from the actual
client network. Comparing with a separate network can locate the failure, but a
successful cellular check does not establish reachability on a blocked office
network.

## Always-on (explicit opt-in)

On a fresh setup, `gtmux tunnel` runs in the **foreground**; Ctrl-C stops that
foreground tunnel. If an always-on tunnel is already loaded, the command instead
prints its existing pairing address and exits. Background remote access is an
explicit opt-in and starts at user login, subject to network availability:

- `gtmux tunnel --service` — for Standard, provisions the tunnel and registers two
  per-user **LaunchAgents** (`com.gtmux.serve` → `gtmux serve` on loopback;
  `com.gtmux.tunnel` → `cloudflared` with the connector token), `RunAtLoad` +
  `KeepAlive`. It explains the standing exposure and asks first (`--yes` bypasses
  the prompt — used by the menu-bar toggle, which shows its own confirmation).
- `gtmux tunnel --unservice` — unloads and removes the shared serve agent and
  either backend's tunnel agent. It does not stop a separately started foreground
  tunnel.
- `gtmux tunnel --status` — on/off + the stable URL.
- The connector token lives in the tunnel plist (0600). The menu-bar app surfaces
  remote-access controls and status. Without an explicit backend, rerunning
  `--service` preserves an already-installed Direct backend;
  `--backend cloudflare --service` selects Standard explicitly.

## Delivered safeguards and remaining work

The Standard provisioner has best-effort per-IP/global creation caps and a daily
unused-tunnel reaper. The reaper removes qualifying tunnel/DNS resources, not
registration KV records. See the [provisioner README](../../tunnel-worker/README.md)
for the checked-in thresholds. There is no `DELETE /provision` route.
The menu-bar pairing sheet and always-on controls have shipped. Application-layer
E2E encryption is still unimplemented.

## Providers: Standard and Direct/self-hosted

`--backend cloudflare|self` and `GTMUX_TUNNEL_BACKEND` select a backend. A fresh
setup defaults to Cloudflare. Both depend on DNS, reachable endpoints and network
policy; Direct's use of HTTPS does not make it unblockable.

- **Standard (`cloudflare`)** uses the hosted Cloudflare path above.
- **Direct/self-hosted (`self`)** connects an in-process jpillora/chisel client to
  a selected HTTPS server. No separate chisel client executable is needed on the
  Mac. The server's TLS proxy forwards a per-Mac `/p<port>` path through chisel to
  serve; [self-tunnel setup](../../deploy/self-tunnel/README.md) contains the
  versioned Caddy/nginx configurations. For your own server, configure
  `GTMUX_SELFTUNNEL_URL` and `GTMUX_SELFTUNNEL_SECRET` (`user:pass`), or persist them
  in `~/.config/gtmux/selftunnel.conf`. The installed `gtmux tunnel-client` service
  needs that configuration too; the generated plist does not carry the secret.
  Hosted Direct obtains a per-Mac account through `gtmux tunnel --redeem <code>`.

The [July self-hosted proposal](../../openspec/changes/archive/2026-07-12-self-hosted-tunnel/proposal.md)
records P1 and the work deferred then. Since that proposal, the embedded chisel
client, paid Direct enrollment, server selection and paired-client route discovery
have shipped. The QR still carries one address; paired clients learn alternate
Direct addresses from `/api/addresses`. This is not automatic switching from
Cloudflare to Direct. The original cross-backend failover/dual-URL QR proposal
remains unimplemented.

## Debug runbook (pairing / reachability)

See [TROUBLESHOOTING.md](../TROUBLESHOOTING.md) for dated incidents and detailed
checks. Current diagnostic starting points:

1. **An enrollment code fails.** It may have expired, already been used, or been
   minted before serve restarted. Mint a fresh code after serve is stable. If
   that still fails, `lsof -nP -iTCP:8765 -sTCP:LISTEN` can reveal conflicting
   listeners. IPv4 and IPv6 paths must reach the same intended serve; inspect
   process ownership before stopping any process.
2. **The connector repeatedly reports QUIC errors.** Check the selected protocol.
   gtmux defaults to `http2`; `GTMUX_TUNNEL_PROTOCOL` overrides it. A previously
   installed plist retains its old arguments until the service is installed
   again. Offline status alone does not diagnose QUIC blocking.
3. **Provisioning succeeds but the phone cannot connect.** Check DNS and the
   public `/api/health` route from the phone's network as well as connector state.
   A separate-network success narrows the fault; it does not fix the first path.

## Code map

| Piece | Where |
|---|---|
| CLI hosted + quick modes, cloudflared runner, QR | `internal/app/tunnel.go` |
| Build-time API URL + reg gate (env-overridable) | `internal/app/tunnelconfig.go` |
| Control-plane Worker (provision via CF API) | `tunnel-worker/src/index.ts` |
| Deploy config (account/zone/KV ids, domain) | `tunnel-worker/wrangler.toml` |
| Reg-gate injection | `Makefile`, `.goreleaser.yaml`, `.github/workflows/release.yml` |
| Capability spec | `openspec/specs/remote-access/spec.md` |
