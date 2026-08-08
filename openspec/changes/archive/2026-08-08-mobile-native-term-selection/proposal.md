# Change: Native iOS terminal text selection (UITextInteraction over the colored grid)

**Status: APPROVED (user picked "直接上 A", 2026-08-08) — design settled, implementation in stages.**

## Why

Selecting/copying text from the mobile terminal on iOS has burned four shipped
attempts, each device-tested (2026-08-07/08):

1. `<Text selectable>` (dual transparent overlay): iOS RN Text selection is
   **menu-only** — no band, no handles, Copy grabs the whole buffer.
2. Always-on read-only UITextView overlay: the band painted **off the touch
   point** (UITextView's TextKit layout drifts from RN Text's on wrapped CJK
   lines) and the 1000-line relayout on every poll **janked typing/scrolling**.
3. Full-screen select sheet (slice from pressed line / from estimated first
   visible line): estimate drift on wrapped lines → the sheet auto-scrolled
   ("上翻好几次") / pressed line yanked to the top / dead black region.
4. In-place surface, selection extends only DOWN from the pressed line: solid
   geometry (measured, wrap-proof) but **"only drag down" is absurd** next to
   native selection. User verdict: 反常识.

Root cause across all four: iOS native-quality selection (band + BIDIRECTIONAL
handles + loupe + edit menu) comes only from UITextView (which owns its own
layout → misalignment) or from **UITextInteraction attached to a view that
implements `UITextInput`** — where WE supply the selection geometry. Option A
(chosen): implement that view. It is the only path where system-native
selection UI draws directly over OUR colored rendering with zero alignment
risk.

## What Changes

**Stage 1 — make the terminal a true uniform grid (JS, enables everything):**
- Explicit uniform `lineHeight` on terminal rows (kills CJK/Menlo line-height
  variance — one poison of every alignment bug so far).
- Pre-wrap logical lines into VISUAL ROWS in JS by cell arithmetic (CJK = 2
  cells, conservative capacity so a row can never native-wrap; extend
  termLineCache to cache rows per line). Each rendered block = exactly one
  visual row → row geometry is `y = index * rowH`, pure arithmetic.
- Side benefit: hard-wrap matches how tmux itself wraps panes (terminals
  char-wrap, not word-wrap) — closer to the Mac's real rendering.

**Stage 2 — the native module (Swift, iOS only):**
- `TermSelectionView`: a transparent UIView overlaid on the row stack (inside
  the same scroll content). Props: rows (plain text per visual row), rowHeight,
  font name/size, content width. Implements `UITextInput` (read-only subset:
  positions/ranges/offsets, `text(in:)`, `caretRect`, `selectionRects`,
  `closestPosition`, `characterRange(at:)`, `UITextInputStringTokenizer`) with
  geometry: row = `floor(y / rowH)`; in-row x from a lazily-built, cached
  `CTLine` per row — Core Text advances are exact and identical to RN Text's
  rendering (same shaping engine, same font; Menlo bold is metrically
  identical; CJK fallback advances measured, not assumed 2×cell).
- `UITextInteraction` + `UIEditMenuInteraction` (iOS 16+; fleet devices are
  far newer): long-press → system word selection + band + both-direction
  handles + loupe; Copy puts the selected range on the clipboard.
- Registered as a legacy RCTViewManager component (works on new-arch via
  interop) — the app already carries native modules (LiveActivity).
- The JS select-sheet (attempt 3/4 code) is deleted; Android keeps its working
  `<Text selectable>` overlay path untouched.
- Poll freezing while a selection is active reuses the existing freeze/thaw.

## Impact

- Affected specs: `mobile-app` (terminal viewer: selection requirement).
- Affected code: `mobileapp/src/ui/NativeTerm.tsx`, `mobileapp/src/ui/term.ts`
  (+cache), NEW `mobileapp/ios/TermSelection/*.swift` (+ project wiring).
- Risk: Stage 1 changes terminal visuals (uniform line height, char-wrap) on
  BOTH platforms — verify on device before Stage 2. Stage 2 is iOS-only.
- Out of scope: Android changes; deeper-than-2000-line history; edge
  auto-scroll while dragging a handle past the viewport (v1: scroll, then
  continue the drag — selection persists).
