# Tasks

## Design first (no code)
- [x] `/design` the Direct server chooser: the list with round-trip figures and the server
      in use, a server's own detail (its address, the one action that fits it), and what
      moving says it costs
- [x] the phone's side of a move: what a user sees while the app is trying the other
      addresses, and what the "can't reach" message says when none answer
- [x] fold the outcome into `docs/design/DESIGN.md` (menu bar) and `MOBILE.md` (phone,
      iPad), both halves of each

## Slice 1 — a server is a row of configuration
- [x] tunnel-worker: server records in the store (id, url, region, label en/zh, accepting,
      restricted), with the single configured address as the implicit server when no list
      exists
- [x] tunnel-worker: `POST /direct/servers` answers the list a caller may use (a POST, so a code never rides in a URL)
- [x] tunnel-worker: `POST /direct/redeem` takes a server id or region preference and names
      the server it assigned
- [x] tunnel-worker: the account file endpoint authenticates per server and answers with
      that server's accounts only
- [x] tunnel-worker: an operator script to add, edit and disable a server without a deploy
- [x] tunnel-worker tests: assignment by region, a restricted server withheld, per-server
      account files, a legacy deployment with no list behaving as today
- [x] client: record which server this device is on; `gtmux tunnel --servers` lists them
      with a measured round trip
- [x] each server answers a liveness path with no Mac behind it (both front-end shapes)
- [x] docs: `docs/cli.md` + `.zh.md`, the command table, CLAUDE.md's command list
- [ ] adding a server changes no code: verified by adding a second one with the script
      alone and redeeming onto it

## Slice 2 — moving without re-pairing
- [x] tunnel-worker: `POST /direct/move` authenticated by the device's own account, a plain
      reassignment with no window
- [x] client: `gtmux tunnel --server <id>` moves, waiting for the new server to hold the
      account before it reconnects
- [x] serve: `GET /api/addresses` (this Mac's port on each server it may use, current
      first) for an authenticated client, and `api/contract.md`
- [x] mobile: fetch that list on connect, try the others when the saved address stops
      answering, save the one that worked (phone and iPad, one code path)
- [x] `gtmux attach`: follow a moved Mac through the same list
- [x] menu bar: the list, a server's detail, the move and what it says it costs, per the
      design
- [x] tests: a client follows a move it slept through; a client that never connected after
      pairing is told to scan again; a probe against a server the Mac is not on reaches no
      one; each guard verified by putting its defect back
- [ ] docs: what a move costs (a device that never connected after pairing, guest links),
      in the user docs, both halves

## Installer (needed by slice 1, does not touch any machine here)
- [x] domain, server id and sync token as parameters
- [x] a front-end mode for a box whose 443 already belongs to an existing nginx: a site
      template, no second web server installed
- [x] a download prefix (a mirror) and a locally prepared file as fallbacks for a box that
      cannot reach GitHub, with the pinned SHA-256 verified before install in every case
- [x] test: a source that serves the wrong bytes is refused, not installed
- [x] `deploy/self-tunnel/README.md`: adding a server to the pool

## Operator, when they choose to (not performed by this change)
- [ ] DNS for the new server's host name
- [ ] run the installer on that box
- [ ] write the server list and per-server sync tokens in the provisioner's store
- [ ] decide whether that server accepts devices other than the operator's own
