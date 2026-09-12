# Change: ipad-universal-app

> STATUS: proposed 2026-09-12. Design canvas: https://claude.ai/code/artifact/b4c7d610-88cb-4b2a-b58f-18dbe789b914
> (iPad page = the deliverable; "HQ directions" page = the two alternatives not taken).
> Design record: `docs/design/MOBILE.md` §5 (rewritten by this change) + `design.md` here (D1–D11).

## Why

The iPad layout was designed and partly built in 2026-07 (MOBILE §5, `SplitScreen`,
`ui/layout.ts`) and then deferred at the first App Store submission. What shipped is an
iPhone-only target (`TARGETED_DEVICE_FAMILY = 1` on all three targets), so the split
view has never reached a device. Picking it back up as-is would ship the shape it was
left in, and that shape has the defect the commander named when asking for this change:
it drifts. `SplitScreen` is a 267-line copy of `RadarScreen`'s chrome (connection dot,
banners, collapse persistence, i18n summary) with its own selection state that no other
part of the app can see. Every radar change since 2026-07 that did not also touch
`SplitScreen` widened the gap; the floating HQ disc, the errored section, the guest
banner and the long-press row sheet all landed on the phone only.

Beyond the radar, nothing else has a wide-canvas layout: the HQ page, the pane browser,
the sheets and the composer all render as a stretched phone. And none of the things an
iPad has that a phone does not (a hardware keyboard, a pointer, Split View and Stage
Manager window sizes, four orientations) are handled.

## What Changes

1. **Universal target.** The app, the widget and the notification service build for
   iPhone and iPad from one target; the plist already carries the iPad orientation set.
2. **One shell, two presentations.** A size-class hook decides `compact` (the phone's
   stack, unchanged) or `regular` (a sidebar + a main pane). Screens become content the
   shell places; they stop knowing which shell they are in.
3. **No copies.** The radar's chrome is extracted into one `RadarPanel` that the phone
   screen and the sidebar both render; `SplitScreen` is deleted. Detail, HQ and All
   panes are the same components on both shells with a layout prop. A structural test
   fails the build if a second radar chrome appears.
4. **Selection is workspace state.** What is open (a pane, HQ, All panes) lives in a
   context; the compact shell maps it to navigation, the regular shell to the main
   pane. Push deep links and keyboard shortcuts drive the selection, never a shell.
5. **Wide layouts where the width changes the reading:** HQ page = report header +
   console + a calls/acts inspector; All panes = a grid of session cards; Knowledge =
   list | entry inside the sheet; chat and the HQ console cap at a reading width, the
   terminal never does.
6. **iPad extras:** hardware keyboard commands (a native `UIKeyCommand` bridge and one
   keymap table), pointer hover, a collapsible sidebar that survives Stage Manager
   resizes, no orientation lock, Split View / Slide Over allowed (compact under 768×600).
7. **Store:** a 13" iPad screenshot set drawn by the existing demo-mode pipeline on the
   iPad Pro 13" simulator; an iPad lane in the e2e harness; store notes.

## Surfaces

- 终端 (terminal / attach)：不适用 —— 排布与输入设备的改动，CLI 和远程 attach 没有对应物。
- 菜单栏 (menubar)：不适用 —— 菜单栏已经是常驻侧栏式的窗口；知识库「列表 | 正文」是它先有的，iPad 借过来。
- 手机 (phone)：不变，compact 壳一字不动；`RadarPanel` 抽取后手机渲染的是同一份。
- iPad：本变更的主体。Demo 模式在 iPad 上也走 regular 壳（Phase 2b）。
- Web：不适用 —— 共享页是只读镜像，没有雷达 / HQ 页。

## What does NOT change

- The phone's screens, navigation and gestures. Compact is the phone, byte for byte.
- The breakpoint: `isSplitCanvas` (width ≥ 768 and height ≥ 600) stays, with its
  reasons (iPhone landscape is not a small iPad).
- The serve API, the push relay, the design language (status triple-encoding, colour =
  status, minimal motion).
- Multi-window (scenes) and Apple Pencil annotation stay out of scope (D9).

## Impact

- `mobileapp/src/ui/layout.ts` (+ `useSizeClass`), new `src/state/WorkspaceContext.tsx`,
  new `src/screens/RadarPanel.tsx`, `src/screens/SplitShell.tsx` (replaces
  `SplitScreen.tsx`), `App.tsx` route choice, `DetailScreen`/`HQScreen`/
  `PaneBrowserScreen`/`KnowledgeSheet` layout props, a native `KeyCommands` module.
- `ios/GtmuxMobile.xcodeproj/project.pbxproj` device family on three targets.
- `scripts/frame-shots.mjs`, `e2e/__tests__/appstore-shots.test.ts`,
  `fastlane/screenshots/*/ipad/`, `e2e/setup/capabilities.ts` docs.
- Spec: `openspec/specs/mobile-app/spec.md` (delta in `specs/`). Docs: MOBILE §5.
