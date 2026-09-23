# Direct: a Mac on several servers at once, and each device picks its own

## Why

`direct-server-choice` gave the fleet a pool of Direct servers and let the operator's Mac
choose one. A Mac is on ONE of them at a time, so "which route" is a property of the Mac,
decided at the Mac, for everybody.

That is the wrong owner for the decision. The route that is good is not the same for every
device: the Mac sits still, its phone does not. A phone on a train in another country, or
on a network that treats one host badly, meets a route that is slow while the Mac's own
measurement of it is fine. Today that phone has nothing to do about it.

Asking the Mac to move would answer it badly. One device would drag every other device
onto a server chosen for one of them, and everything paired to that Mac would drop for the
seconds the move takes. A shared, disruptive change is the wrong shape for "this phone,
right now, is on a bad route".

## What changes

**A Mac holds a tunnel on every server it may use, not one.** The reverse port is already
unique across the whole fleet, so the same `/p<port>` means that Mac on every server. The
Mac dials each one and keeps them; an idle tunnel costs a connection and no traffic.

**A device picks its route locally.** Every client already learns the addresses its Mac
answers at (`GET /api/addresses`). It measures them, uses the one it likes, and can change
its mind in a second, with no request to the Mac and no effect on any other device. The
phone shows the route it is on and lets the user pick another, which is the whole point of
this change.

**Credentials become per device AND per server.** Today a device has one account, on one
server. Multi-homing must not mean handing every server one credential that works on all of
them: that undoes the isolation `direct-per-device-accounts` exists for. The provisioner
mints a separate `user:pass` for each (device, server) pair, all bound to that device's one
port, and each server's account file carries only its own.

**The pairing code still carries ONE address.** A QR holds what it holds. The code carries
the Mac's preferred route, and the list arrives over the connection the scan establishes,
exactly as it does now.

**Failover stops being a feature.** With several routes live, a server going down is a
client-side retry, not an event anyone has to handle. The Mac does nothing, and no address
anyone stored becomes wrong.

## What this replaces

`gtmux tunnel --server <id>` moves a Mac from one server to another. After this it names
the PREFERRED route, which is what the pairing code carries and what a client uses until it
has a reason not to. Nothing is taken away from the menu bar; the sentence under the
chooser changes, because moving no longer disconnects anybody.

## Cost, stated

- The Mac holds N tunnels instead of one. Each is a WebSocket carrying nothing when unused.
- Each Direct server carries a bound port per Mac that may use it, whether or not any device
  is currently going through it. That is what makes an instant client-side switch possible.
- The provisioner stores N credentials per device instead of one.

## Not in this change

- **Choosing a route automatically**, by measurement, without the user. Worth doing once
  people have used the manual pick and we know what they actually switch for.
- **Per-route scopes or pricing.** A route is a path to the same Mac, not a tier.

## Surfaces

- **终端 / terminal**: `gtmux tunnel --servers` marks every route this Mac is on and which
  one is preferred; `--server <id>` sets the preference rather than moving. `gtmux attach`
  keeps using its address list, which now has more than one live entry.
- **菜单栏 / menubar**: the server list shows every route as live, with its round trip, and
  the chooser sets the preferred one. The sentence about what a move costs goes away, since
  nothing drops.
- **手机 / phone**: shows the route it is on and lets the user pick another, measured from
  the phone. The switch is local and instant; nothing else on the fleet notices.
- **iPad**: identical to the phone, one implementation.
- **Web**: the shared page keeps the address it was opened with. A browser cannot measure
  and re-pick without a page of its own, and a guest link is for watching a pane; out of
  scope, and stated so rather than left to be discovered.
