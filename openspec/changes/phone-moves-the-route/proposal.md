# Direct: the paired phone can move its Mac to another route

## Why

Which Direct route a Mac uses is decided at the Mac. The person who most often meets a bad
route is not at the Mac: they are somewhere else, holding the phone, watching a pane redraw
slowly. Today that phone can see which route it is on and nothing else.

The paired app is the owner's Mac at a distance. It types into panes, sends work, reads the
supervisor's board. Choosing the route is the same kind of act performed from the same
place, and treating it as one keeps the product's existing line: a PAIRED device is the
owner and may do what the owner may do; a SHARE link is a guest, scoped to panes, and sees
none of this.

**A first draft proposed multi-homing instead** — the Mac on every server at once, each
device picking its own route locally. It is dropped. It answers a different question (each
device wanting a different route), and pays for that answer three times: a tunnel per
server per Mac, a bound port on every server for every entitled Mac, and a credential per
device per server. The product this is for has one owner, and the owner's choice is the
Mac's.

## What changes

**serve answers the owner's two questions.** `GET /api/routes` lists the Direct servers this
Mac may use and says which one carries it; `POST /api/route` moves it. Both are OWNER-only:
a guest token is refused, and a guest surface never shows the section at all.

**The phone measures for itself.** The round trip beside each route is the one THIS DEVICE
measured against that route's address, not the Mac's own figure. A user on the other side of
the world is asking what their own connection costs; the Mac's measurement is a different
question with a different answer.

**Moving from the phone is the same act as moving from the menu bar**, and says the same
things first: every other paired device drops for the seconds the move takes and comes back
by itself, a device that paired but never connected has to scan again, and guest links
minted before the move stop working.

**The phone can trust what it sees afterwards.** It already keeps every address its Mac
answers at and tries the others when one goes quiet, so after the move it finds the Mac on
the new route by itself and says which one it ended up on.

## Not in this change

- **Choosing a route automatically.** Worth doing once there is evidence of what people
  switch for; a manual pick is how that evidence appears.
- **Per-device routes.** Rejected above, with the reason, so the idea is not re-proposed
  from scratch later.
- **Moving a Mac that cannot be reached.** If the route is down rather than slow, the phone
  cannot ask the Mac anything; that case is automatic failover's, not this change's.

## Surfaces

- **终端 / terminal**: unchanged. `gtmux tunnel --servers` / `--server <id>` already do this
  locally, and the new endpoints are the same operation behind serve.
- **菜单栏 / menubar**: unchanged, and it stays the fullest surface: it is where a route is
  chosen when the user is at the Mac.
- **手机 / phone**: a route list under the connection, each with the round trip this phone
  measured, and a picker that moves the Mac after naming what it costs. Owner only.
- **iPad**: identical to the phone, one implementation and one code path.
- **Web**: none. The shared page is a guest surface, and a guest may not move someone
  else's Mac; the section is absent rather than refused.
