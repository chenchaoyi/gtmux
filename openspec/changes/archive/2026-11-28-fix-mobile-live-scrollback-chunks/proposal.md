# Proposal: Bounded Native Layers for Mobile Live Scrollback

## Why

PR #1954 (`e58e4c8d`) expanded mobile Chat Live scrollback from the visible row slice to `MOBILE_SCROLLBACK_LINES` (1,000 lines), joining all 1,000 lines into a single `<Text selectable>` (`RCTParagraphTextView`). Because `RCTParagraphTextView` overrides `-drawRect:`, CoreAnimation allocates a single non-tiled `CABackingStore` / Metal 2D texture across the entire paragraph bounds. On `@3x` iPhones, any capture past ~303 lines (`5,461pt` = `16,384px`, and `1,000` lines = `18,000pt` = `54,000px`) exceeds Apple GPU / Metal's `16,384px` 2D texture ceiling (`@2x` iPads and Android HWUI hit the same ceiling past ~455 lines), causing texture creation to fail and the expanded Live preview to render completely blank.

## What Changes

- Partition `LiveScrollback` output in `mobileapp/src/ui/LiveProgress.tsx` into bounded vertical `<Text selectable>` chunks (`LIVE_CHUNK_MAX_ROWS = 32`) inside an explicit vertical column `<View>` within the horizontal scroll container.
- Split pathological lines wider than `LIVE_CHUNK_MAX_COLS = 320` terminal cells via `wrapLine` while leaving normal wide terminal rows (`<= 320` cells) unwrapped for horizontal scrolling, keeping both width and height of every native paragraph layer safely below GPU texture limits.
- Update paired mobile design docs (`docs/design/MOBILE.md` and `docs/design/MOBILE.zh.md`) and prepare `v1.0.206` release metadata across iOS and Android (`Antigravity` official icon & widget support + Live View long-scrollback rendering fix).

## Surfaces

- **Terminal**: Unchanged (`capture-pane -p -e -S -2000` remains read-only and unmodified).
- **Menubar**: Unchanged (macOS menu-bar popover does not host Chat Live scrollback).
- **Phone**: Fixes blank expanded Live View on panes with >300 scrollback lines on iOS and Android phones; prepares `1.0.206` release notes and version stamp.
- **iPad**: Shares `ChatView` and `LiveProgress` in Detail and HQ Console; fixes the same >455-line `@2x` GPU texture overflow.
- **Web**: Unchanged (browser `<pre class="live-output">` renders DOM text without a single `CALayer` bitmap limit).

## Impact

- Modified code: `mobileapp/src/ui/LiveProgress.tsx`, `mobileapp/src/ui/LiveProgress.chunks.test.tsx`.
- Modified docs & release metadata: `docs/design/MOBILE.md`, `docs/design/MOBILE.zh.md`, `mobileapp/fastlane/metadata/*/release_notes.txt`, `mobileapp/release-notes/*`, `mobileapp/src/version.ts`, `mobileapp/src/releaseNotes.ts`, `mobileapp/ios/GtmuxMobile.xcodeproj/project.pbxproj`.
