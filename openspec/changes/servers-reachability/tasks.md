# Tasks

## Design
- [x] Commander chose the layout (2026-10-05): a check mark for the open Mac, plus a status
      line on every row; the radar title becomes `● name ⌄` with a brand chevron
- [x] MOBILE.md and MOBILE.zh.md: the server-list paragraph and the radar top bar

## Phone and iPad
- [x] `serverReachability`: probe each Mac's `/api/health` while the page is shown, on
      arrival and every 15s, with a timeout; a pure status function for one row
- [x] Servers rows: fixed two lines, check slot, status line (dot plus words), pending
      setting merged into the status line, no "Updating…" and no Retry sync
- [x] Automatic resync when a probe finds a pending Mac answering again
- [x] Radar title: dot before the name, brand chevron after it, one target; the ⇄ square goes
- [x] en and zh strings

## Tests
- [x] status function: every open and probed state, pending merged, guests
- [x] Servers screen: check mark only on the open Mac; two lines whatever the state; no
      Retry control; pending text on the status line; resync when a pending Mac answers
- [x] probe: answers, fails, times out; stops when the page goes
- [x] radar title: chevron present, no ⇄, dot before the name, tap opens Servers

## Ship
- [ ] %6 simulator check, before and after: the two layers, no jump on bell or switch
- [ ] sync specs and archive this change
