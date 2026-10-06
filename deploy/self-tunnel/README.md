# gtmux self-hosted tunnel — server setup (VPS)

The self-hosted tunnel backend ("Direct") connects a Mac over **443 / WebSocket**
to a **VPS + domain**. It avoids the Standard tunnel's Cloudflare edge path; it does
not guarantee access on every network. The Mac and its remote clients must be able
to reach the selected server. This directory contains reference server configs and
an installer; deployment-specific DNS, credentials and front-end configuration are
operator-managed.

> **Two ways to get Direct.** (1) **gtmux's paid Direct** — a hosted server; the app/CLI
> unlocks it with an access code (`gtmux tunnel --redeem <code>`), which the control-plane
> Worker validates server-side and hands back the config. The server + secret are **never**
> baked into the (public) binary — that's what keeps this repo fully open-source. (2) **Your
> OWN server** — use a VPS meeting the prerequisites below, then point the client at it via
> `GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET` (or `~/.config/gtmux/selftunnel.conf`).
> The personal setup below is for (2); the Direct-mode and server-pool sections are
> for operators running their own provisioner and account registry.

## Architecture (dedicated VPS — Caddy owns :443)

```
public :443 ─► Caddy (Let's Encrypt TLS for tunnel.ccy.dev)
                 ├─ WebSocket   ─► 127.0.0.1:8080  chisel server (each Mac's control channel)
                 ├─ /p<port>/…  ─► 127.0.0.1:<port> (per-Mac reverse-forward → that Mac's serve:8765)
                 └─ else (legacy)─► 127.0.0.1:9000  (single-tenant / personal client)
public :80  ─► Caddy (ACME HTTP-01 only)
Mac ─► chisel client  https://tunnel.ccy.dev  R:127.0.0.1:<port>:localhost:8765   (assigned or derived port)
```

`tunnel.ccy.dev` above is the hostname in the checked-in Caddyfile, not a hostname
your own VPS can use. Replace it in the copied Caddyfile with a domain you control
and point that domain at your server. The Caddy installer copies this file verbatim;
`DOMAIN` is only used by the nginx branch.

The default setup assumes a **dedicated VPS** where Caddy can bind :443 and :80 directly. If
your box already runs something else on :443, you'd front Caddy with your own SNI/port
router — that's out of scope here (keep such host-specific config in your own private
ops, not in this public reference).

## Direct mode: one account per device

A server shared by more than one person must NOT run on one shared chisel secret. chisel
gives an `--auth` user every permission, so each holder could bind any other Mac's port
(receiving that Mac's phone's token when it slept) and open forward tunnels to every
loopback service on the VPS, Caddy's admin API included. gtmux's operated Direct server
uses **Direct mode** (see the [current remote-access spec](../../openspec/specs/remote-access/spec.md)):

- A redeem creates or reuses that device's account in the provisioner Worker, allowed to bind
  exactly `R:127.0.0.1:<its port>`.
- `gtmux-authsync.timer` pulls chisel's authfile from the Worker every 10s
  (`GET /direct/authfile`, using that server's sync token) and applies accepted changes
  atomically. The timer interval is not a revocation deadline: fetching or applying a
  response can fail. A failed fetch, malformed file, or a rule of `""`/`*` keeps the
  existing file. A response that removes the last device account needs a complete-set
  claim, a matching account count and the previously recorded server ID; see the upgrade
  and last-account rules below.
- chisel runs on that authfile alone (`chisel-server.direct.conf`), as its own user. There
  is no `AUTH`: an `--auth` user is pinned with every permission and cannot coexist.
- **The authfile is never empty.** chisel with no users switches authentication OFF — anyone
  connects and no rule applies. Every file carries a sentinel account whose password exists
  only on this server (`/etc/gtmux-tunnel/sentinel`) and whose rule (`^$`) matches nothing.
- **Applying a removal restarts chisel.** Reloading the authfile alone does not close
  established tunnels. After an accepted removal, authsync requests a restart to end them;
  remaining devices can reconnect if their network path is available. Additions do not
  request a restart. Before replacing a file that removes accounts, authsync records
  `/etc/gtmux-tunnel/restart-pending`. Each later sync retries that restart before fetching,
  even if the next fetch fails or the account list is unchanged. Only a successful restart
  clears the marker. New accounts can still be applied while a restart is owed, but that
  sync exits with failure until the restart succeeds. Check both sync and restart results
  before treating a revocation as complete.
- The legacy `/…` → `:9000` route has nothing behind it in this mode: no account may bind 9000.

Install or convert a server to Direct mode (it stays in Direct mode on later re-runs):

```sh
DIRECT_SYNC_URL='https://provisioner.example.com/direct/authfile' \
DIRECT_SYNC_TOKEN='REPLACE_WITH_THIS_SERVER_SYNC_TOKEN' \
  bash /tmp/gtmux-self-tunnel/install-server.sh
```

For your own provisioner, set `DIRECT_SYNC_URL` to its `/direct/authfile` endpoint;
the installer otherwise defaults to gtmux’s operated API. The sync token must match
the server entry in that same provisioner. The nginx example below needs the same
override when using your own provisioner.

On a box whose :443 belongs to something else (an SNI router in front of Caddy, with a
Caddyfile of its own), add `CADDY=skip`: the script skips replacing the Caddyfile and
restarting Caddy. It still installs Caddy if absent. This flag does not set up your
front-end routing.

## More than one server: a pool the client chooses from

A deployment may run several Direct servers, and which one a Mac uses is the user's choice
(`gtmux tunnel --servers`, `gtmux tunnel --server <id>`). **A server is a row of
configuration, never a release:** the provisioner keeps the list, clients ask for it at run
time, so a server added today is selectable by installations that already exist.

```sh
cd tunnel-worker
./direct-servers.sh list
./direct-servers.sh add 'SERVER_ID' 'https://tunnel.example.com' 'REGION' 'English label' '中文名称'
./direct-servers.sh set 'SERVER_ID' accepting false     # stop giving it new devices
./direct-servers.sh set 'SERVER_ID' codes 'CODE_1,CODE_2' # reserve it for named codes only
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
FRONT=nginx DOMAIN='tunnel.example.com' DIRECT_SYNC_TOKEN='REPLACE_WITH_THIS_SERVER_SYNC_TOKEN' \
  bash /tmp/gtmux-self-tunnel/install-server.sh
```

It writes `/etc/nginx/sites-available/gtmux-direct.conf` (from `nginx-site.conf`), leaves
every other site untouched, checks the config and RELOADS nginx rather than restarting it.
Before writing, `nginx-site-install.sh` saves the previous site file and enabled entry
outside nginx's included directories. If the new configuration fails its check, it restores
that site and entry (including the target of an enabled symlink, or an enabled regular file);
if neither existed, it removes the new ones. It checks the restored configuration and
reloads only if that check passes. If it still fails, the restored files remain in place
and nginx is not reloaded. Other site files are not rewritten.

Certificates come from certbot on the nginx host. If the expected certificate is
absent, the installer writes an HTTP-only site so certbot has somewhere to attach.
Run `certbot --nginx -d tunnel.example.com` for your actual domain, then re-run the
installer for TLS. When that domain’s certificate already exists, it uses the TLS
site directly. The installer does not install or run certbot.

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

`tunnel-worker/revoke-direct-code.sh <code>` removes the code and its device accounts
from the registry. The VPS must then accept the changed authfile and successfully restart
chisel. The timer normally runs every 10 seconds, but that is not a deadline: fetches,
validation and restarts can fail. Read that server's `gtmux-authsync` logs and check the
restart result; a registry update alone does not prove that an open tunnel ended.

### Upgrading authsync and accepting the last removal

These rules require the updated provisioner Worker **and** the updated server-side
`gtmux-authsync`. A CLI/App release, or merging these files into the repository, deploys
neither. Operators must separately deploy their Worker and install the server script.
This README does not attest that either update is running on a particular server.

The Worker returns `X-Gtmux-Authfile: complete; server=<id>; accounts=<n>` only when it
read an existing registry with an accounts object and can name the server. Authsync checks
the body first, verifies that a recognised claim's count matches it, and records that claim's
server ID in `/etc/gtmux-tunnel/server-id` only if no ID has been recorded yet. A valid
nonempty complete response can establish this pin even if the account file itself did not
change. Once pinned, a complete response naming another server is refused whether it is
empty or nonempty; neither the account file nor the pin is replaced.

A change from one or more device accounts to zero is accepted automatically only when
that complete claim names the already recorded server ID. The resulting file still has
the deny-all sentinel; it is never a zero-user chisel authfile. A missing or malformed
registry, a failed response, an absent or unrecognised claim, a mismatched account count,
or an empty response naming a different server cannot authorise that last removal.
An empty response before an ID has ever been recorded cannot authorise it either.
Failed fetches and rejected responses leave the current account file in place; a pending
restart is still retried before the fetch.

Either side can be upgraded first. An old authsync ignores the new header and keeps its
old refusal of a last-account removal; an old Worker supplies no claim, so the new authsync
also refuses that removal by default. To complete an operator-approved upgrade:

1. Deploy the updated Worker using your existing provisioner deployment procedure. Install
   the reviewed `gtmux-authsync` script at `/usr/local/bin/gtmux-authsync` on each intended
   VPS, retaining its server-specific `sync.env` and sentinel. Review the installer before
   choosing to re-run it: it also replaces managed front-end configs and restarts services.
2. On a server with accounts remaining, let a successful complete sync record `server-id`.
   Check it against that VPS's intended provisioner server entry. Do not copy another
   server's ID into the file to make an empty answer pass. If the server ID intentionally
   changes, first independently verify the intended server entry and sync configuration;
   only then remove `server-id`. The next accepted complete sync can establish the new pin.
   If accounts remain on the VPS but that next answer is empty, removing the pin alone is
   insufficient: the last-account protection below still applies.
3. If the registry is already empty while this VPS still holds accounts and has no ID,
   automatic sync intentionally stays blocked. An operator must verify, through their
   existing private registry/server records, that the sync URL and token select this exact
   VPS and that **all** its device accounts are meant to be gone. A missing registry or
   an unexplained change of server identity is not that confirmation.
4. Only after that confirmation, the operator may use the existing `ALLOW_EMPTY=1`
   override for one sync: stop the timer and any in-flight sync service, privately back up
   `sync.env`, add or change that setting in the root-only file, then run authsync. Inspect
   the sync/restart outcome and restore the previous setting before re-enabling the timer,
   even if the attempt failed. `sync.env` is sourced after the process environment, so a
   setting in that file takes precedence over a command-line environment override. This
   override permits an empty set without the normal complete-claim/pin match; it cannot
   override a complete claim for a different existing pin, bypass body validation or remove
   the sentinel. An owed restart
   must remain marked and be retried until it succeeds. This is a manual migration choice,
   not a step the client update performs.

The new retry mechanism cannot discover an old restart failure that happened before
`restart-pending` existed. For that case, after checking the intended authfile, an operator
must successfully restart chisel once, or wait for another accepted removal to create a
pending restart. A restart interrupts the other devices' existing tunnels too; their
reconnection depends on their network path.

## Multi-tenant routing

**Multi-tenant.** Operated Direct assigns a port in 20000–59999, starting with the
CRC32-derived preference and choosing the next free port on collision. The Mac uses the
saved assignment and pairs at `https://<your-host>/p<port>`. A personal setup without an
assignment falls back to the derived port; the hash alone is not collision-free.
Caddy strips `/p<port>` and proxies to that loopback port. If registry updates race and
produce a collision, the authfile generator emits only one account for that port. The port matcher is confined to the chisel band (`[2-5]\d{4}`),
and every serve is bearer-token-gated. The bare `/…` (no `/p<port>`) still routes to
the legacy fixed 9000 for a pre-multi-tenant client or a one-Mac personal server.

## Prerequisites

1. **Your domain and DNS:** create an A record for your chosen hostname pointing at
   the VPS IP, with DNS-only / grey-cloud routing to avoid the Cloudflare proxy path.
   Replace `tunnel.ccy.dev` in the copied Caddyfile before installation. DNS and network
   reachability must be ready for certificate issuance.
2. Debian 12 **amd64** VPS (the downloader selects `linux_amd64`), ports 443 + 80
   reachable from the internet, and root SSH. The default Caddy setup needs both ports
   available; use the nginx branch or your own front-end configuration when shared.
3. Personal mode only: the shared secret (`AUTH=user:pass`) is generated at install into
   `/etc/gtmux-tunnel/chisel.env` (0600) and mirrored to the Mac — **never committed**.

## Install / update

```sh
scp -i 'SSH_KEY_PATH' -r deploy/self-tunnel root@VPS_HOST:/tmp/gtmux-self-tunnel
ssh -i 'SSH_KEY_PATH' root@VPS_HOST 'bash /tmp/gtmux-self-tunnel/install-server.sh'
```

Replace the example domain and uppercase placeholders with your own values before
running commands. Copy to a fresh staging directory; repeated `scp -r` into an existing
directory can create a nested copy. Review the staged Caddyfile before running the installer.

`install-server.sh` can be re-run: it installs the selected front end and chisel,
replaces its managed configs, preserves an existing personal secret, and restarts
services. It is not a dry run and does not preserve custom edits to managed files.

The downloader pins chisel **1.12.0 linux_amd64** and replaces an installed version
that differs from the pin. `CHISEL_MIRROR=<prefix>` prepends a mirror to the GitHub
archive URL; both downloaded paths must pass `verify-download.sh` against the pinned
archive SHA-256. `CHISEL_BIN=/path/to/chisel` instead copies a supplied executable
**without that checksum check**: its provenance, version and architecture are the
operator's responsibility. Do not treat the local-binary override as verified by the
archive hash.

Direct relies on chisel's ACLs. The [upstream ACL-bypass advisory](https://github.com/jpillora/chisel/security/advisories/GHSA-24fp-5v3p-rvpw)
identifies **1.11.5** as the fix for GO-2026-5054; the current installer pin includes
that fix. This is a pin, not a claim about the newest upstream release.

The original deployment notes recorded client/server interoperability checks for
1.10.1 and 1.12.x on **2026-09-22**. That historical result is not a current security
endorsement of an older server or a test of every later release.

## The Mac side

`gtmux tunnel --backend self` reads the Direct config (from `--redeem <code>`, or
`GTMUX_SELFTUNNEL_URL` + `GTMUX_SELFTUNNEL_SECRET` for your own server) and runs the
chisel client, reverse-forwarding this Mac's **per-device port** (see multi-tenant
above). To test manually — using the LEGACY fixed `:9000` (single-tenant) path:

```sh
AUTH='REPLACE_WITH_USER:REPLACE_WITH_PASSWORD' chisel client --keepalive 25s \
  https://tunnel.example.com R:127.0.0.1:9000:localhost:8765
```

This manual example requires a separately installed `chisel` command and
`gtmux serve` already listening on the Mac’s port 8765. The normal gtmux client embeds
the chisel library; it does not install a standalone `chisel` binary.
Pair to your own base URL and serve token for this legacy 9000 route. The regular CLI
uses the `/p<port>` address; a Direct per-device account cannot bind the legacy 9000 port.

## Migrating to a new VPS

For a **personal** server, prepare the replacement with your chosen hostname and the
same personal `chisel.env` (or regenerate the secret and update the Mac), then move DNS
and verify TLS and the Mac connection. Keep copied credentials private.

A **Direct-mode** server instead needs its server-specific sync configuration and
account registry relationship; copying `chisel.env` does not migrate it. Follow the
Direct-mode setup, confirm account synchronization and TLS, then switch traffic. A new
server ID or hostname also affects the provisioner and clients; do not treat a DNS change
alone as a complete Direct migration.

## Rollback / off

In Direct mode, stop the sync timer and any in-flight sync service before stopping
chisel, so a later account removal cannot restart it:

```sh
systemctl disable --now gtmux-authsync.timer
systemctl stop gtmux-authsync.service
systemctl disable --now chisel-server
```

Personal mode has no authsync timer; stop just `chisel-server`. Stop Caddy only if it is
dedicated to this tunnel. On a shared nginx host, remove only this site's enabled link,
validate the remaining config and reload using your normal server procedure. These steps
turn access off; they do not restore overwritten configuration or erase credentials.

## Files

| File | Installs to | Role |
|---|---|---|
| `Caddyfile` | `/etc/caddy/Caddyfile` | TLS for the server's host name → chisel (owns :443) |
| `nginx-site.conf` | `/etc/nginx/sites-available/gtmux-direct.conf` | the same routing as one nginx site, for `FRONT=nginx` |
| `nginx-site-acme.conf` | the same path, first run | HTTP only, so certbot has a server block to attach a certificate to |
| `chisel-server.service` | `/etc/systemd/system/` | chisel reverse-tunnel endpoint |
| `chisel-server.direct.conf` | `/etc/systemd/system/chisel-server.service.d/direct.conf` | Direct authfile and restricted service user |
| `gtmux-authsync` | `/usr/local/bin/gtmux-authsync` | validate and apply the provisioner's account file |
| `gtmux-authsync.service` / `.timer` | `/etc/systemd/system/` | run authsync periodically |
| `verify-download.sh` | — | verify the downloaded archive; not the `CHISEL_BIN` override |
| `install-server.sh` | — | installer for the selected mode/front end |
| `nginx-site-install.sh` | — | its `FRONT=nginx` step: installs the site, checks, reloads; on a failed check puts the previous site and link back |
