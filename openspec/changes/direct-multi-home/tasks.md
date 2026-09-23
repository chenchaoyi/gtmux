# Tasks

## Design first (no code)
- [ ] `/design` the phone's route picker: where it lives, how a route is shown as fast or
      slow from THIS device, and what a switch looks like when it is instant and local
- [ ] the menu bar's list once every route is live: what "preferred" means on screen

## Provisioner
- [ ] a credential per (device, server), all bound to that device's one port
- [ ] redeem and move return every route a device may use, with its own credential
- [ ] each server's account file carries only its own credentials
- [ ] tests: one server's credential does not authenticate on another; a device keeps its
      port everywhere; an older client that expects a single account still works

## The Mac
- [ ] dial every route the provisioner returned, and keep them
- [ ] `--servers` marks live routes and the preferred one; `--server` sets the preference
- [ ] `GET /api/addresses` reports every LIVE route, preferred first
- [ ] tests: a route that fails to dial does not stop the others

## Phone and iPad
- [ ] measure the routes and show what each one costs from this device
- [ ] pick one, instantly, with nothing asked of the Mac
- [ ] tests: switching changes only this device; a route that stops answering falls back

## Docs
- [ ] `docs/cli.md` + `.zh.md`, `api/contract.md`, `docs/design/DESIGN.md` + `MOBILE.md`
      and both `.zh.md` halves
- [ ] the user docs say what a route is and who it belongs to (the device, not the Mac)
