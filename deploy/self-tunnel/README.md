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

## Direct mode: one account per device

A server shared by more than one person must NOT run on one shared chisel secret. chisel
gives an `--auth` user every permission, so each holder could bind any other Mac's port
(receiving that Mac's phone's token when it slept) and open forward tunnels to every
loopback service on the VPS, Caddy's admin API included. gtmux's operated Direct server
therefore runs in **Direct mode** (`openspec/changes/direct-per-device-accounts`):

- Each redeem mints that device its own account in the provisioner Worker, allowed to bind
  exactly `R:127.0.0.1:<its port>`.
- `gtmux-authsync.timer` pulls chisel's authfile from the Worker every 10s
  (`GET /direct/authfile`, bearer `DIRECT_SYNC_TOKEN`) and swaps it in atomically. A failed
  fetch, a malformed file, a rule of `""`/`*`, or dropping every account at once keeps the
  file on disk.
- chisel runs on that authfile alone (`chisel-server.direct.conf`), as its own user. There
  is no `AUTH`: an `--auth` user is pinned with every permission and cannot coexist.
- **The authfile is never empty.** chisel with no users switches authentication OFF — anyone
  connects and no rule applies. Every file carries a sentinel account whose password exists
  only on this server (`/etc/gtmux-tunnel/sentinel`) and whose rule (`^$`) matches nothing.
- **Revoking restarts chisel.** It re-reads the authfile for new sessions, but does not
  interrupt established tunnels, and a device's reverse port is one; the restart ends every
  session, each device reconnects in seconds, and a removed one cannot. Additions reload
  without a restart.
- The legacy `/…` → `:9000` route has nothing behind it in this mode: no account may bind 9000.

Install or convert a server to Direct mode (it stays in Direct mode on later re-runs):

```sh
DIRECT_SYNC_TOKEN=<this server's own token> bash /tmp/gtmux-self-tunnel/install-server.sh
```

On a box whose :443 belongs to something else (an SNI router in front of Caddy, with a
Caddyfile of its own), add `CADDY=skip`: the script then leaves Caddy exactly as it is.

## More than one server: a pool the client chooses from

A deployment may run several Direct servers, and which one a Mac uses is the user's choice
(`gtmux tunnel --servers`, `gtmux tunnel --server <id>`). **A server is a row of
configuration, never a release:** the provisioner keeps the list, clients ask for it at run
time, so a server added today is selectable by installations that already exist.

```sh
cd tunnel-worker
./direct-servers.sh list
./direct-servers.sh add <id> https://<host name> <region> "<label en>" "<label zh>"
./direct-servers.sh set <id> accepting false     # stop giving it new devices
./direct-servers.sh set <id> codes gtd-…,gtd-…   # reserve it for named codes only
```

`add` prints that server's OWN sync token once. Each server gets its own, and receives
only the accounts assigned to it: a server never holds the credentials of devices that do
not use it. Put the token in that box's install (`DIRECT_SYNC_TOKEN=…`).

A Mac moving between servers keeps its account and its port, so only the host name in its
pairing address changes. Paired phones follow it there; a device that paired but never
connected has to scan again, and guest links minted before the move have to be re-minted.

Every server answers `GET /__gtmux/ping` with 204, and no Mac is behind that path: it is
how a client times a server it has no account on, and how you check a server with no device
paired to it.

## A box whose :443 already belongs to an existing nginx

The files above assume Caddy owns the site. A box already serving other sites through nginx
takes one extra nginx site instead, and no Caddy at all:

```sh
FRONT=nginx DOMAIN=<host name> DIRECT_SYNC_TOKEN=<this server's token> \
  bash /tmp/gtmux-self-tunnel/install-server.sh
```

It writes `/etc/nginx/sites-available/gtmux-direct.conf` (from `nginx-site.conf`), leaves
every other site untouched, checks the config and RELOADS nginx rather than restarting it.
If that check fails, the gtmux site is removed again and nginx is left as it was.

Certificates come from the certbot already on such a box, in two steps: the first run
installs an HTTP-only site so certbot has somewhere to attach, you run
`certbot --nginx -d <host name>`, and re-running the installer then serves it over TLS.

nginx sees the visitor directly here, so it passes the real address on, and the device
roster shows where each device actually connected from.

### Cutover from the shared secret (operator, in order)

1. Deploy the Worker (`cd tunnel-worker && npx wrangler deploy`) and set
   `wrangler secret put DIRECT_SYNC_TOKEN` to a new random value.
2. Run the command above on the VPS with that value. chisel is upgraded to 1.12.0, the
   timer is enabled, and chisel restarts on the authfile: the shared secret stops working
   here.
3. On each Mac that used the shared secret: `gtmux tunnel --redeem <code>` (codes are not
   used up, so the same code works), then turn Direct back on.
4. `wrangler secret delete DIRECT_SECRET`.

Revoke a code and every device it unlocked: `tunnel-worker/revoke-direct-code.sh <code>`.

## Multi-tenant routing

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
3. Personal mode only: the shared secret (`AUTH=user:pass`) is generated at install into
   `/etc/gtmux-tunnel/chisel.env` (0600) and mirrored to the Mac — **never committed**.

## Install / update

```sh
scp -i <key> -r deploy/self-tunnel root@<VPS>:/tmp/gtmux-self-tunnel
ssh -i <key> root@<VPS> 'bash /tmp/gtmux-self-tunnel/install-server.sh'
```

`install-server.sh` is idempotent: installs caddy + chisel, drops the configs,
generates the secret if absent, and (re)starts the services.

It pins a chisel version and re-running it upgrades a server that differs. A box that
cannot reach GitHub (measured on a mainland host: the release download truncates or times
out) takes `CHISEL_MIRROR=<prefix>` or `CHISEL_BIN=<path to a binary you carried over>`.
Either way the pinned SHA-256 is checked before anything is installed — that is what makes
a mirror safe to use, since the trust is in the checksum and not in whoever served the
bytes; `verify-download.sh` is the guard, and a Go test runs it against bytes that do not
match. The server is
on 1.12.0 (the newest with a release binary; 1.12.1, which the CLI links, is a module tag
only, and the two differ only in client-side UDP forwarding). It has to be at least
1.11.5: GO-2026-5054 is an ACL bypass, and Direct mode is what gives the ACL work to do.

Clients and servers of 1.10.1 and 1.12.x interoperate in every combination (checked
2026-09-22), so the server and the Macs can be upgraded in either order.

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
| `Caddyfile` | `/etc/caddy/Caddyfile` | TLS for the server's host name → chisel (owns :443) |
| `nginx-site.conf` | `/etc/nginx/sites-available/gtmux-direct.conf` | the same routing as one nginx site, for `FRONT=nginx` |
| `nginx-site-acme.conf` | the same path, first run | HTTP only, so certbot has a server block to attach a certificate to |
| `chisel-server.service` | `/etc/systemd/system/` | chisel reverse-tunnel endpoint |
| `verify-download.sh` | — | the pinned checksum, checked before anything downloaded is installed |
| `install-server.sh` | — | idempotent installer |
