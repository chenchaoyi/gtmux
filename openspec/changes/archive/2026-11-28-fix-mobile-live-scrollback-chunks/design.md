# Design: Bounded Native Layers for Mobile Live Scrollback

## Context

In React Native 0.86 Fabric on iOS, `<Text>` mounts `RCTParagraphComponentView` backed by `RCTParagraphTextView : UIView`, which draws via `-drawRect:`. UIKit/CoreAnimation creates a single non-tiled backing store (`width * contentsScale` by `height * contentsScale`). Apple Metal GPUs reject 2D textures exceeding `16,384px` in either dimension (`5,461pt` at `@3x`, `8,192pt` at `@2x`), and Android HWUI enforces `8,192px`–`16,384px` hardware layer limits. Joining 1,000 terminal lines (`18,000pt` = `54,000px` at `@3x`) into one `<Text>` caused expanded Live View to display an empty `#17171A` viewport on real panes with normal scrollback depth.

## Decisions

1. **32-row memoized `<Text>` chunks in a vertical column `<View>`**:
   - Group bounded rows into slices of `LIVE_CHUNK_MAX_ROWS = 32` rows (`~576pt` = `1,728px` at `13pt` `@3x`, well below both `8,192px` and `16,384px` even under `2x` Dynamic Type).
   - Wrap the chunk list in a column `<View>` inside `<ScrollView horizontal>` so horizontal scroll layout never places chunks side-by-side.
   - Coalesce adjacent styled spans within each chunk and memoize `<LiveChunk>` by row-slice content so unchanged leading chunks avoid re-rendering while new lines append before buffer rotation, and offscreen clipped chunks outside the `220pt` viewport need not rasterize upfront.

2. **320-cell pathological line ceiling (`LIVE_CHUNK_MAX_COLS = 320`)**:
   - Rows up to 320 terminal cells (covering standard 80–240 column tmux panes, with an `O(1)` code-unit length fast path `<= 160`) remain single unwrapped rows so horizontal scrolling works identically.
   - Only rows exceeding 320 cells pass through `wrapLine(row, 320)` from `term.ts`, preserving ANSI color/bold, CJK cell widths, variation selectors, and surrogate pairs while keeping chunk width (`<= ~2,505pt` = `7,516px` at `@3x`) below GPU texture bounds.
