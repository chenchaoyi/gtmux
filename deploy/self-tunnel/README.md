# gtmux self-hosted tunnel — server setup (VPS)

The self-hosted "anywhere" tunnel backend: a Mac dials out over **443 / WebSocket**
to **your own VPS + domain**, indistinguishable from ordinary HTTPS, so hostile
networks that DNS-hijack Cloudflare's tunnel edge (`*.argotunnel.com`) can't block
it. This directory is everything that runs on the VPS, versioned so the server can
be rebuilt / migrated from scratch.

## Architecture (443 shared with an existing VLESS-REALITY proxy)

```
public :443 ─► nginx (stream + ssl_preread, SNI router, TLS passthrough — no decrypt)
                 ├─ SNI tunnel.ccy.dev ─► 127.0.0.1:4443  Caddy (Let's Encrypt TLS)
                 │                                          ├─ WebSocket ─► 127.0.0.1:8080  chisel server
                 │                                          └─ else      ─► 127.0.0.1:9000  (chisel reverse-forward → Mac serve:8765)
                 └─ SNI else / www.ivi.tv ─► 127.0.0.1:8443  xray VLESS-REALITY (your proxy)
public :80  ─► Caddy (ACME HTTP-01 only)
Mac ─► chisel client  https://tunnel.ccy.dev  R:127.0.0.1:9000:localhost:8765
mail 25/110/143/993/995 (postfix/dovecot) and SSH 22 — UNTOUCHED
```

Both TLS services terminate their OWN TLS; nginx only peeks the SNI and splices the
raw TCP, so xray's REALITY camouflage is preserved and Caddy gets a real cert.

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

`install-server.sh` is idempotent: installs nginx+stream / caddy / chisel, drops the
configs, generates the secret if absent, and (re)starts the services. It does NOT
touch mail or SSH. Bring xray back under the router with `xray-integrate.sh` (see
below) once its config is fixed.

## The Mac side

`gtmux tunnel --backend self` (once P1 ships) reads `GTMUX_SELFTUNNEL_URL` +
`GTMUX_SELFTUNNEL_SECRET` and runs the chisel client. To test manually before that:

```sh
AUTH=<user:pass> chisel client --keepalive 25s \
  https://tunnel.ccy.dev R:127.0.0.1:9000:localhost:8765
```

Then pair the phone to `https://tunnel.ccy.dev` + the serve token.

## Migrating to a new VPS

1. Point `tunnel.ccy.dev` A record at the new VPS IP (DNS-only).
2. Run the install steps above on the new box.
3. Copy `/etc/gtmux-tunnel/chisel.env` over (or regenerate + update the Mac).
4. If the new box also runs VLESS-REALITY, run `xray-integrate.sh`; else drop the
   `default` line from `nginx-stream-sni.conf` (all SNIs → Caddy).

## Rollback / off

```sh
systemctl disable --now chisel-server caddy
rm /etc/nginx/stream.d/gtmux-sni.conf && systemctl reload nginx   # or stop nginx
# xray returns to owning :443 by reverting its inbound port 8443 → 443
```

## Files

| File | Installs to | Role |
|---|---|---|
| `nginx-stream-sni.conf` | `/etc/nginx/stream.d/gtmux-sni.conf` | SNI passthrough router on :443 |
| `Caddyfile` | `/etc/caddy/Caddyfile` | TLS for tunnel.ccy.dev → chisel |
| `chisel-server.service` | `/etc/systemd/system/` | chisel reverse-tunnel endpoint |
| `install-server.sh` | — | idempotent installer |
| `xray-integrate.sh` | — | fix xray's geoip error + move its inbound to 127.0.0.1:8443 |
