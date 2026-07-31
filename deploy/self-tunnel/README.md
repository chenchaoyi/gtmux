# gtmux self-hosted tunnel — server setup (VPS)

The self-hosted "anywhere" tunnel backend ("Direct"): a Mac dials out over **443 /
WebSocket** to a **VPS + domain**, indistinguishable from ordinary HTTPS, so hostile
networks that DNS-hijack Cloudflare's tunnel edge (`*.argotunnel.com`) can't block
it. This directory is everything that runs on the VPS, versioned so the server can
be rebuilt / migrated from scratch.

> **Two ways to get Direct.** (1) **gtmux's paid Direct** — a hosted server; the app/CLI
> unlocks it with an access code (`gtmux tunnel --redeem <code>`), which the control-plane
> Worker validates server-side and hands back the config. The server + secret are **never**
> baked into the (public) binary — that's what keeps this repo fully open-source. (2) **Your
> OWN server** — stand up this directory on any VPS, then point the client at it via
> `GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET` (or `~/.config/gtmux/selftunnel.conf`).
> The steps below are for (2).

## Architecture (dedicated VPS — Caddy owns :443)

```
public :443 ─► Caddy (Let's Encrypt TLS for tunnel.ccy.dev)
                 ├─ WebSocket   ─► 127.0.0.1:8080  chisel server (each Mac's control channel)
                 ├─ /p<port>/…  ─► 127.0.0.1:<port> (per-Mac reverse-forward → that Mac's serve:8765)
                 └─ else (legacy)─► 127.0.0.1:9000  (single-tenant / personal client)
public :80  ─► Caddy (ACME HTTP-01 only)
Mac ─► chisel client  https://tunnel.ccy.dev  R:127.0.0.1:<port>:localhost:8765   (port derived from device id)
```

These files assume a **dedicated VPS** where Caddy can bind :443 and :80 directly. If
your box already runs something else on :443, you'd front Caddy with your own SNI/port
router — that's out of scope here (keep such host-specific config in your own private
ops, not in this public reference).

**Multi-tenant.** Each Mac derives a STABLE per-device port in 20000–59999 (crc32 of
its device id) and pairs at `https://tunnel.ccy.dev/p<port>`; Caddy strips the
`/p<port>` prefix and proxies to that loopback port. So several Macs share ONE gtmux
Direct server without colliding on a fixed port — a phone always reaches the Mac whose
`/p<port>` it scanned. The port matcher is confined to the chisel band (`[2-5]\d{4}`),
and every serve is bearer-token-gated. The bare `/…` (no `/p<port>`) still routes to
the legacy fixed 9000 for a pre-multi-tenant client or a one-Mac personal server.

## Prerequisites

1. **DNS (DNS-only / grey cloud):** `tunnel.ccy.dev  A  <VPS-IP>` — must NOT be
   proxied through Cloudflare (the whole point is to bypass it, and Cloudflare's
   proxy also breaks the long-lived connection). Required before Caddy can issue a cert.
2. Debian 12 VPS, ports 443 + 80 reachable from the internet, root SSH.
3. The shared secret (`AUTH=user:pass`) is generated at install into
   `/etc/gtmux-tunnel/chisel.env` (0600) and mirrored to the Mac — **never committed**.

## Install / update

```sh
scp -i <key> -r deploy/self-tunnel root@<VPS>:/tmp/gtmux-self-tunnel
ssh -i <key> root@<VPS> 'bash /tmp/gtmux-self-tunnel/install-server.sh'
```

`install-server.sh` is idempotent: installs caddy + chisel, drops the configs,
generates the secret if absent, and (re)starts the services.

## The Mac side

`gtmux tunnel --backend self` reads the Direct config (from `--redeem <code>`, or
`GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET` for your own server) and runs the
chisel client, reverse-forwarding this Mac's **per-device port** (see multi-tenant
above). To test manually — using the LEGACY fixed `:9000` (single-tenant) path:

```sh
AUTH=<user:pass> chisel client --keepalive 25s \
  https://tunnel.ccy.dev R:127.0.0.1:9000:localhost:8765
```

Then pair the phone to `https://tunnel.ccy.dev` + the serve token.

## Migrating to a new VPS

1. Point `tunnel.ccy.dev` A record at the new VPS IP (DNS-only).
2. Run the install steps above on the new box.
3. Copy `/etc/gtmux-tunnel/chisel.env` over (or regenerate + update the Mac).

## Rollback / off

```sh
systemctl disable --now chisel-server caddy
```

## Files

| File | Installs to | Role |
|---|---|---|
| `Caddyfile` | `/etc/caddy/Caddyfile` | TLS for tunnel.ccy.dev → chisel (owns :443) |
| `chisel-server.service` | `/etc/systemd/system/` | chisel reverse-tunnel endpoint |
| `install-server.sh` | — | idempotent installer |
