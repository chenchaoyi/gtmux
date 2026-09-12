# Tasks — ipad-universal-app

## Phase 0 — design
- [x] 0.1 Canvas: sidebar + detail, HQ page, All panes, Knowledge, portrait; HQ directions A/B/C
- [x] 0.2 MOBILE §5 rewritten (size classes, no drawer, composition rule, keyboard, store)
- [x] 0.3 This change: proposal, design D1–D11, spec delta

## Phase 1 — the shell
- [x] 1.1 `useSizeClass()` in `ui/layout.ts`; `layout.test.ts` covers the hook's inputs
- [x] 1.2 `WorkspaceContext` (selection + select); compact shell maps to navigation
- [x] 1.3 `RadarPanel` extracted from `RadarScreen`; `RadarScreen` renders it; sidebar renders it
- [x] 1.4 `SplitShell` (sidebar 300/280, collapse toggle persisted, main pane by selection) replaces `SplitScreen.tsx`
- [x] 1.5 Detail: reading-width cap for chat; terminal uncapped; no back button on regular (HQ and All panes likewise)
- [x] 1.6 `TARGETED_DEVICE_FAMILY = "1,2"` on all three targets
- [x] 1.7 `shellDrift.test.ts` (D3) + `WorkspaceContext` tests (the push deep-link now goes through `select`, covered there) + `split-shell` e2e on the iPad Pro 13" simulator

## Phase 2 — wide layouts
- [ ] 2.1 HQ page: route wrapper + view; regular = header + console + inspector (D5)
- [ ] 2.2 All panes: grid of session cards on regular; search field in the header
- [ ] 2.3 Knowledge sheet: list | entry on regular (D11)
- [ ] 2.4 Pointer hover tint on rows and buttons (D8)

## Phase 3 — keyboard
- [ ] 3.1 `KeyCommands.swift` native bridge (UIKeyCommand → event)
- [ ] 3.2 `src/keys/keymap.ts` + dispatcher; every binding in D7
- [ ] 3.3 Tests: keymap complete, ids unique, actions resolve; e2e on the iPad sim sends key commands

## Phase 4 — store and verification
- [ ] 4.1 e2e iPad lane (env only) + `split-shell` e2e on iPad Pro 13" sim with screenshots
- [ ] 4.2 `frame-shots.mjs` slot parameter; `appstore-shots` on the iPad sim; `fastlane/screenshots/*/` iPad set
- [ ] 4.3 Store notes + What's New; `set-version.sh` gate green after the stamp
- [ ] 4.4 Device build on an iPad (or the sim if none) through the documented xcodebuild command
- [ ] 4.5 Sync specs, archive this change
