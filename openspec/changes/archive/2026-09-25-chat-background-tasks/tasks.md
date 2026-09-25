# Tasks

## serve
- [x] `GET /api/tasks`, owner-only, guest refused with 403
- [x] the join in the app layer: ledger + one radar pass, `gone` for a pane that is no
      longer there, never dropped
- [x] `joinTasks` split out as a pure function so the join is testable without tmux
- [x] `api/contract.md`
- [x] tests, each confirmed by re-introducing its defect (guest gate, gone task, since
      fallback)

## The phone's model
- [x] `src/api/backgroundTasks.ts`: what counts as running (waiting counts), the tally,
      the row's words, the sort, whether a row can be opened, the duration format
- [x] `client.tasks()`, with a guest 403 reading as an empty list
- [x] tests, each confirmed by re-introducing its defect (waiting folded into finished,
      the row burying what needs you, a gone pane offering navigation)

## The UI
- [x] `RunningRow`: a control, not a status line — the approval card's own language (8%
      tint, 30% border, radius 13) plus a chevron, so colour still says state only
- [x] `TasksSheet`: two groups, collapsible, finished counted; a row leads to its pane; a
      gone task is dimmed with no chevron
- [x] sheet conventions from MOBILE.md: `surface` over `bg` with `raised` controls, the
      cap in points from `useWindowDimensions`, no Touchable as the container
- [x] wired into HQ's chat only, polled at 20s, both languages
- [x] the demo shell shows real shapes

## Verify
- [x] `npm run check`, `make check`, `check-design.sh`, all by exit code
- [x] sync-specs + archive
