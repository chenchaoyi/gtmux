# Tasks — menubar-hq-report

## Core
- [x] 1.1 `gtmux events --acts` (filter via `events.IsSupervisorAct`); usage en/zh; unit test
- [x] 1.2 `docs/cli.md` + `docs/cli.zh.md` events section; session-events spec delta

## Menu bar
- [x] 2.1 `ResourceReport` keeps the machine readings + orphan count; `AgentStore` exposes them
- [x] 2.2 Pure report model (`HQCardReport.swift`): rows in both languages, order, tones, absence; acts tally; auto-open rule; tests
- [x] 2.3 Card expansion in `MenuView`: disclosure, table, doors, remembered toggle, auto-open on red
- [x] 2.4 Reader window "Machine" tab: readings, warning sentence, per-agent table, orphans (read-only)

## Docs / specs
- [x] 3.1 DESIGN.md + DESIGN.zh.md §12; mockup artboards under `docs/design/mockup/menubar-hq/`
- [x] 3.2 menu-bar-app + session-events specs synced; change archived
- [x] 3.3 Gates green (`make check`, `scripts/check-design.sh`, `swift build -c release && swift test` 184/184); the real-menu-bar smoke in both languages is done at install, not in the PR (a second GtmuxBar beside the running one is exactly the duplicate this repo warns about)
