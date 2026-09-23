# Tasks

## Design first (no code)
- [ ] `/design` the phone's route list: where it lives under the connection, how a route
      reads when it is fast, slow or silent FROM THIS PHONE, and what the confirmation says
- [ ] what the phone shows while the Mac is moving, and when it has followed it

## serve
- [ ] `GET /api/routes` (owner only): the routes this Mac may use, and the one in use
- [ ] `POST /api/route` (owner only): move this Mac, the same operation the CLI performs
- [ ] a guest token is refused on both, and `api/contract.md` says so
- [ ] tests: a guest is refused; an unknown route id is refused; the move reaches the same
      code path as the CLI's

## Phone and iPad
- [ ] the route list, with the round trip measured from THIS device
- [ ] the confirmation, naming what a move costs, before anything happens
- [ ] after the move: follow to the new route and say which one it is on
- [ ] the section is absent for a guest connection
- [ ] tests: the list renders from what serve returns; a guest sees nothing; each guard
      verified by putting its defect back

## Docs
- [ ] `api/contract.md`, `docs/phone.md` + `.zh.md`, `docs/design/MOBILE.md` + `.zh.md`
