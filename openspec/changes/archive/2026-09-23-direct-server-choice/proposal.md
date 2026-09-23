# Direct: a choice of servers, added by configuration

## Why

Direct is one server. Every buyer's Mac dials it and every phone reaches its Mac through
it, wherever any of them are. Two things follow.

**Distance is not negotiable.** A user whose Mac, phone and work are all in one region
still pays for a round trip to wherever that one server is. Measured from the operator's
Mac on 2026-09-22: a TLS handshake of about 0.21s to the current Direct server, about
0.12s to a box in the same country. Every keystroke in `gtmux attach`, every pane read,
every send carries that difference.

**One server is one outage.** When it is down, or its network is, every Direct user is
offline at once with nothing to switch to. There is no second address to try and no way
to tell them about one.

Nothing in the product can express "another server". The provisioner holds a single
address; the installer writes one domain into the server's config; the Mac stores one
address; and every paired phone, iPad and guest link stores an address with that domain
inside it. So a second server is not a setting today, it is a fork of the whole chain.

**Adding a server must be configuration, not a release.** The operator expects to add
Direct servers over time, on whatever VPS is convenient. A new server must therefore be a
row of data the provisioner serves, visible to clients that are already installed. No
Worker code change, no CLI release, no app release, and nothing about any particular
server baked into the binary.

## What changes

**The provisioner keeps a list of servers.** One record per server: an id, its base URL, a
region code, a label in both languages, whether it accepts new devices, and whether it is
offered to everyone or only to named codes. The list lives in the provisioner's store, not
in its source, and a small operator script adds, edits and disables rows. A deployment
with no list behaves exactly as today: the single configured Direct address is the one
implicit server, so nothing that exists now has to be migrated.

**Clients learn the servers at run time.** `GET /direct/servers` answers the list a caller
may use. The CLI and the menu bar render whatever it returns, so a server added on a
Tuesday is selectable that afternoon by every Mac already installed. No client ever holds
a built-in server list.

**Redeeming picks a server.** `POST /direct/redeem` accepts a server id or a region
preference and returns which server the device got, alongside the account and port it gets
today. A device's port stays globally unique across the whole fleet, so it is the same
number on every server: moving changes the host name in the pairing address and nothing
else.

**A device can move.** `POST /direct/move`, authenticated by the device's own account,
reassigns it to another server. The Mac drops its tunnel, waits for the new server to have
its account (one sync, seconds), and reconnects there. Nothing runs in parallel and no
window has to be waited out.

**Each server gets only its own accounts.** The account file endpoint authenticates per
server, and answers with the accounts assigned to that server and no others. Today one
shared sync token hands every server every account's password; with more than one server
that is a blast radius, not a detail.

**Each server answers a liveness path** that no Mac sits behind, so a client can time it
and the operator can check a server without a paired device. That is what the menu bar's
round-trip figure is measured with.

**Moving keeps devices paired, by candidates rather than by timing.** `GET /api/addresses`
tells an authenticated client every address this Mac could answer at: its port on each
server it may use, current one first. The phone, iPad and `gtmux attach` fetch that list on
their first connection and refresh it on every later one, and when the saved address stops
answering they try the others before reporting the Mac unreachable.

Nothing has to happen while the Mac is moving, which is why this is the whole mechanism:
a device that was asleep for a week finds the Mac the same way a device that was open
finds it. Trying another server is safe because a device's port is unique across the fleet:
no one else can be behind `/p<port>` on any server, so a probe that finds nothing finds
nothing, and never hands the token to a stranger.

The pairing code still carries ONE address and stays as small as it is now: a QR's module
count is the constraint there, and the list arrives over the connection the scan
establishes.

**The installer stops assuming one deployment shape.** The server's domain, its id and its
sync token become parameters. Two shapes are supported: the current one, where Caddy owns
the site, and a box whose 443 already belongs to an existing nginx, which gets a site
template instead of a second web server.

**Where the transport binary comes from becomes a parameter too, and its checksum is
checked whatever the source.** Measured on 2026-09-23 from a mainland box: GitHub's release
download either truncated mid-file or returned nothing in 40 seconds, while two public
mirrors delivered the same file in 1 to 5 seconds, byte-identical to the published
checksum. So the installer takes a download prefix and, failing that, a locally prepared
file, and verifies the pinned SHA-256 before installing either. That verification is what
makes a mirror safe to use: the trust is in the checksum, not in whoever served the
bytes.

## What a move costs

Stated, because a user will meet it:

- The Mac is unreachable for a few seconds while it reconnects on the new server.
- A device that paired but never connected afterwards knows only the one address from its
  pairing code. If the Mac moved in between, that device has to scan again.
- A guest share link carries the address it was minted with. After a move, links minted
  before it stop working and have to be re-minted. A browser page cannot be told where its
  Mac went, and the app is where the address list lives.

## UI

The server chooser, the round-trip figures, the moving state and what the user is told
about already-paired devices are a design question across the menu bar and the phone, not
a layout invented in the implementation. They are designed first, in a design artifact,
and this change follows that design (`docs/design/DESIGN.md` for the menu bar,
`MOBILE.md` for the phone and iPad).

## Considered and dropped: a dual-running window

The first draft moved a Mac by running both tunnels for 30 minutes, so devices could catch
up. Candidate addresses do the same job better and cost far less: they need no window in
the provisioner, no two tunnels at once in the client, no countdown in the UI, and they
cover a device that was away for a week, which a window never could.

## Not in this change

- **Automatic selection and failover.** Picking the fastest server by itself, and moving
  off a server that went down, both need the address-following above to exist first.
  Separate proposal, once moving is proven in use.
- **One host name for every server.** It would need DNS steering plus forwarding between
  servers, and a Mac and its phone could still resolve to different ones. Each server keeps
  its own host name.
- **Per-server pricing or quotas.** A code entitles a device to an account; which server it
  sits on is not a product tier here.

## Operator steps, which are the operator's own

The code change does not perform any of these, and none of them is assumed done:

1. **DNS for a new server's host name.** Only the operator holds the domain.
2. **Standing the server up.** The installer prepares it, but a run touches a machine that
   may already serve a live site, so the operator decides when and runs it.
3. **The server list and its sync tokens** in the provisioner's store.
4. **Whether a given server is offered for sale.** A server can be listed as available to
   the operator's own devices only. This matters for a server whose hosting or domain
   registration does not cover selling access from it; until that is settled, such a server
   ships not accepting new devices.

## Surfaces

- **终端 / terminal**: `gtmux tunnel --servers` lists the servers with a measured round
  trip and marks the current one; `gtmux tunnel --server <id>` moves; `--redeem` takes a
  region preference. `gtmux attach` follows a Mac that moved, through the advertised
  addresses.
- **菜单栏 / menubar**: the Direct section lists the servers with their round trip and marks
  the one in use; opening one shows that server's detail and either moves this Mac there or,
  for the one in use, hands over the pairing code. No address is printed in the list itself.
  Designed first (see UI).
- **手机 / phone**: fetches its Mac's address list on connect and tries the others when the
  saved one stops answering, so a move needs no user action. No new screen; the "can't
  reach" message is the one place the user may see it.
- **iPad**: identical to the phone, one implementation and one code path.
- **Web**: the shared page keeps the address it was opened with. A guest link minted before
  a move must be re-minted; the menu bar says so at the moment of moving, rather than
  leaving a dead link to be discovered.
