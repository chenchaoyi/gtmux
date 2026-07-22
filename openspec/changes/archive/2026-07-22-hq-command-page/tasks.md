# hq-command-page — tasks

## 1. Server: supervisor knowledge endpoints

- [x] 1.1 `GET /api/hq/board` — read HQ's `notes/board.md`, return `{exists, updated_at, text}`; owner-only (guest → 403); absent board is a 200 with `exists:false`
- [x] 1.2 `GET /api/hq/events?severity=&limit=` — severity-floored, newest-first, bounded slice of the event ledger; owner-only
- [x] 1.3 Wire both through `server.Deps` so `internal/server` keeps its layering (no direct `internal/hq` import from the handler)
- [x] 1.4 Tests: guest 403 on both · absent board is not an error · severity floor filters · limit bounds the slice · newest-first ordering

## 2. Mobile client

- [x] 2.1 `client.hqBoard()` / `client.hqEvents()` + types
- [x] 2.2 Demo client equivalents, consistent with the canned world (no blank zones in Demo)

## 3. HQScreen rebuild

- [x] 3.1 Delete the fleet board (list + collapse control + its styles)
- [x] 3.2 Assessment zone: deterministic conclusion line (single-source with `fleetHeadline`) + board freshness row
- [x] 3.3 Situation-board reader (read-only, full-screen, scrollable)
- [x] 3.4 Your-call zone: decision cards from waiting digest rows; ask as the body; open-session + ask-HQ actions; explicit empty state
- [x] 3.5 Activity zone: notable+ event feed as its own full-height zone (the collapse idea was dropped for a zone selector — see the proposal's revised design), unread dot on the tab
- [x] 3.6 Command console retained; target selection moves to decision cards
- [x] 3.7 Tests: zone logic in `hqZones.ts` (assessment text, decision ordering, event prose, unread mark, empty states) — testing the REAL module, replacing a suite that mirrored the board's grouping inside the test file

## 4. Cleanup

- [x] 4.1 Remove the stale `fleet pip strip` comment left in `macapp/.../MenuView.swift` by hq-meta-layer

## 5. Docs + gates

- [x] 5.1 `docs/design/MOBILE.md` §17 rewritten to mirror the new page
- [x] 5.2 `api/contract.md` documents both endpoints
- [x] 5.3 `make check` + `mobileapp: npm run check` + `scripts/check-design.sh` green
- [x] 5.4 openspec validate --specs --strict
- [x] 5.5 sync-specs + archive
