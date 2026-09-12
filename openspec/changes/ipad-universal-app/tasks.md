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
- [x] 2.5 The demo is the real shells over the fake client (SplitShell / RadarPanel with demo chrome); `shellDrift` has no exception left
- [x] 2.6 Five-surfaces rule: `docs/design/SURFACES.md`, CLAUDE.md, `check-design.sh` gate on in-flight proposals, PR template

- [x] 2.1 HQ page: route wrapper + view; regular = header + console + inspector (D5)
- [x] 2.2 All panes: grid of session cards on regular; search field in the header
- [x] 2.3 Knowledge sheet: list | entry on regular (D11)
- [x] 2.4 Pointer hover tint on rows and buttons (D8)

## Phase 3 — keyboard
- [x] 3.1 `KeyCommands.swift` native bridge (UIKeyCommand → event)
- [x] 3.2 `src/keys/keymap.ts` + dispatcher; every binding in D7
- [x] 3.3 Tests: keymap complete, ids unique, actions resolve (`keymap.test.ts`, `KeyCommandBridge.test.ts`). The e2e (`ipad-keys.test.ts`) reaches the app — the bridge registers 22 commands and the main menu is built with them (probe read back) — but XCTest's key injection on the simulator types text and never dispatches a UIKeyCommand (measured 2026-09-12, with and without a first responder, hardware keyboard connected or not), and this Mac's shell has no Accessibility grant for real keystrokes. Dispatch is verified by hand on a device with a keyboard in 4.4; until then the e2e stays gated and expected red on a simulator.

## Phase 4 — store and verification
- [x] 4.1 e2e iPad lane (env only: `GTMUX_E2E_UDID` + `GTMUX_E2E_DEVICE='iPad Pro 13-inch (M5)'`) + `split-shell`, `ipad-demo`, `ipad-keys` e2e on the iPad Pro 13" sim with screenshots
- [x] 4.2 `frame-shots.mjs --slot ipad` (13" landscape 2752×2064, tablet bezel, `--prefix ipad-`); `appstore-shots-ipad` e2e in demo mode with `GTMUX_DEBUG_LANG`; `fastlane/screenshots/*/ipad-0N.png` in both locales
- [ ] 4.3 Store notes + What's New written (both locales lead with the iPad; descriptions mention it); the stamp (`set-version.sh`) and its gate run with the release build, not before
- [ ] 4.4 Device build on an iPad (or the sim if none) through the documented xcodebuild command
- [ ] 4.5 Sync specs, archive this change
