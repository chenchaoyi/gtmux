# Direct: one account per device, each bound to its own port

## Why

Every Direct code redeemed for the SAME thing: the one `DIRECT_SECRET`, a single chisel
user shared by every buyer. The Direct server (`chisel server --reverse` with that one
`AUTH`) gives that user `UserAllowAll`, so anyone holding it can:

1. **Bind any other Mac's reverse port.** Ports are per device (`/p<port>`), but nothing
   ties a port to its owner. When a Mac sleeps its port is free; another holder binds
   it; that Mac's phone keeps polling `/p<port>` with its bearer token and hands the token
   to the impostor. With it, once the victim's Mac is back, `POST /api/send` types into
   the victim's tmux — command execution on someone else's machine. Reproduced locally
   (2026-09-22) with chisel 1.10.1 in the server's exact configuration:
   `A-impostor saw Authorization="Bearer B-phone-token"`. TLS does not help: the phone's
   certificate is tunnel.ccy.dev's, whichever tenant ends up behind it.
2. **Reach every loopback service on the VPS.** The chisel server always accepts
   outbound (forward) tunnels, and `UserAllowAll` matches them, so a holder can connect to
   `127.0.0.1:<anything>` on the VPS — including Caddy's admin API, which the Caddyfile
   leaves on `localhost:2019` and which rewrites the whole edge's configuration.

And revoking a code revokes nothing: the secret is already on the buyer's disk, the only
credential there is, and rotating it cuts off every customer at once.

Measured exposure when this was written: 22 codes, one redemption ever (a Phase 0 manual
code, on a device other than the operator's Mac), and no anonymous buyer holding the
secret yet. The fix is cheapest before the first one does.

## What changes

**The Worker mints a chisel account per device at redeem.** `POST /direct/redeem`
requires `deviceId`, and returns `secret` = that device's own `user:pass` plus the
`port` assigned to it. Accounts live in one KV registry. A device re-redeeming gets its
existing account back; a code is limited to `DIRECT_DEVICES_PER_CODE` devices (default 3,
the cap the ops runbook already planned). A port is the device's preferred one
(`20000 + crc32(deviceId) % 40000`, the formula the client already uses) unless another
device holds it, then the next free one in the band.

**The Worker serves the server's authfile.** `GET /direct/authfile`, authenticated by a
VPS-only `DIRECT_SYNC_TOKEN`, returns chisel's authfile: each account allowed exactly
`^R:127\.0\.0\.1:<its port>$`. No forward tunnel matches that, so the loopback services
are closed too. If two accounts ever hold one port (a lost race), only the older is
emitted: a collision fails closed.

**The VPS runs chisel 1.12.1 on the authfile alone.** No `AUTH`: the `--auth` user is
pinned and `UserAllowAll`, so it cannot coexist with the fix. A timer pulls the authfile
every 10s, validates it, and swaps it in atomically; a failed pull keeps the old one.
1.12.1 is required, not incidental: GO-2026-5054, fixed in 1.11.5, is an ACL bypass, and
this change is what makes the ACL matter.

**The authfile is never empty.** Found while testing this change against a real chisel
1.12.1: with NO users, chisel turns authentication off (`authUser` returns nil when the
index is empty), and with no user attached, no rule is checked either: a stranger binds
any port. A fresh Direct server has no device accounts, and revoking the last one leaves
none, so the cutover as first designed (an initial `{}` authfile) would have opened the
server to the internet at the moment it was meant to close. Every authfile the server
writes carries a sentinel account: a password only the server knows, and the rule `^$`,
which matches no address.

**Revocation becomes real.** `tunnel-worker/revoke-direct-code.sh` deletes the code AND
the accounts it minted. The sync then RESTARTS chisel, because a reload is not enough:
chisel applies a new authfile to new sessions but does not interrupt established tunnels,
and a device's reverse port is one. Measured, a Mac dropped from the authfile kept
serving through its port. After the restart every device reconnects within seconds and a
removed one fails to authenticate. Additions reload without a restart.

**The client keeps the port it was assigned**, and before starting the long-running
tunnel it tries the credentials once: a server that refuses them gets a clear "redeem your
code again" instead of chisel retrying an authentication failure forever in silence.
Clients from before this change keep working with a per-device account (they store any
`secret` they are given and derive the same port), except in the rare case of a port
collision, where they fail closed.

## Cutover (operator, in this order)

1. Deploy the Worker; `wrangler secret put DIRECT_SYNC_TOKEN`.
2. On the VPS: run the updated `install-server.sh` with the same token. It upgrades
   chisel, installs the sync timer, and restarts chisel on the authfile only. From this
   moment the shared secret stops working.
3. Re-redeem each Mac that held it (the operator's; the one Phase 0 device). Codes are not
   consumed, so the same code works.
4. `wrangler secret delete DIRECT_SECRET`.

Codes already sold or stocked on Lemon Squeezy and afdian need no change: a code is an
entitlement to an account, and was never the credential that leaked.

## Measured, 2026-09-22

Against a real chisel 1.12.1 server fed an authfile of the Worker's exact shape
(`internal/app/tunnelaccount_test.go`): a device binds its own port and not another's;
it cannot open a forward tunnel to the server's own services or bind on its public
interface; a revoked device, retrying as the real client does, cannot come back after
the restart; the sentinel keeps a server with no device accounts closed to strangers.
Each guard was checked by putting its defect back and watching the test fail.

## Not in this change

- **Request signing** (the phone never sending a reusable token) protects every transport,
  not just Direct, and is a larger change across four surfaces. Separate proposal.
- **Caddy's admin API** stays on localhost:2019. No tenant can reach it after this change;
  moving it to a unix socket is a hardening for later.
- **KV is eventually consistent.** Two redeems at once in different locations can lose one
  registry write; the device whose account was lost fails to authenticate and re-redeems.
  At this volume that is acceptable; a Durable Object would remove it.

## Surfaces

- **终端 / terminal**: `gtmux tunnel --redeem` stores the assigned port; `--backend self`
  pre-checks the credentials and explains a refusal. The only surface with code changes.
- **菜单栏 / menubar**: none. Its Direct sheet runs `gtmux tunnel --redeem` and shows the
  CLI's stderr, so it inherits the new messages as they are.
- **手机 / phone**: none. It pairs to the same `/p<port>` URL; what changes is who can
  sit behind it.
- **iPad**: none, for the same reason as the phone.
- **Web**: none. The shared page reaches the Mac through the same URL.
