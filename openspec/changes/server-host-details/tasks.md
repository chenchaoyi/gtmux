# Tasks

- [x] 1.1 `internal/hostinfo`: gather names, OS, hardware, boot time and tmux version once, with time-limited probes; pure parsers with tests
- [x] 1.2 `GET /api/host` (owner only, guest 403, 503 without the dependency) + `serve` wiring with gtmux version and serve start; handler tests
- [x] 1.3 `api/contract.md` documents the route
- [x] 2.1 Mobile `client.host()` answers the details or why there are none (old / guest / unreachable); tests
- [x] 2.2 `state/hostInfo`: five-minute cache that keeps a good answer over an unreachable one; labels; tests
- [x] 2.3 Servers page: status-line clause for owned reachable Macs; guests never asked; ••• → Details sheet; demo client answers; tests
- [x] 2.4 en/zh strings; MOBILE.md / .zh.md and docs/phone.md / .zh.md
- [ ] 3.1 Simulator check by %6 (list clause, Details sheet en/zh, guest and unreachable notes)
