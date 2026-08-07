# Tasks — mobile-native-term-selection

## Stage 1 — uniform grid (JS)
- [x] 1.1 Explicit uniform lineHeight for terminal rows (fontSize-derived:
      `rowHeightFor` in term.ts). Shipped iOS-ONLY, deliberately: Android's
      paragraph+overlay path stays byte-identical — its selection works, and the
      grid exists to serve Stage 2's iOS-only native selection layer.
- [x] 1.2 Cell-arithmetic pre-wrap: logical line → visual rows (`wrapLine` +
      `charCells` CJK=2 / selectors+ZWJ=0; `cellWidthFor`/`colsFor` derive a
      conservative row capacity — −1 cell — so a row never native-wraps). Rows
      keyed lineIdx-rowIdx; the JS select-sheet now slices visual rows (source
      line + char offsets deferred to Stage 2's rows→text mapping).
- [x] 1.3 Extend termLineCache: cache parsed spans AND wrapped rows per raw
      line (+ cols/fontSize in the cache signature; both identities stable for
      unchanged lines). Parity test: joined rows == logical line text; no row
      exceeds capacity (termLineCache.test.ts + term.test.ts).
- [ ] 1.4 Device check: grid looks right (CJK alignment, cursor row, wrapped
      long lines), busy-pane typing still smooth.

## Stage 2 — native module (iOS)
- [ ] 2.1 `TermSelectionView.swift`: UITextInput read-only subset over
      rows+rowH+font props; CTLine cache; DocumentPosition/Range types.
- [ ] 2.2 UITextInteraction + UIEditMenuInteraction wiring (long-press select,
      handles both directions, loupe, Copy → clipboard; tap-away clears).
- [ ] 2.3 RCTViewManager registration + JS component `<TermSelection>` mounted
      absoluteFill over the row stack (iOS only); freeze polls while a
      selection is active (reuse freeze/thaw; clear on new snapshot).
- [ ] 2.4 Delete the JS select-sheet path; remove now-dead styles/state.
- [ ] 2.5 Device verification: band under finger everywhere in the buffer
      (deep history + tail), bidirectional handles, loupe, Copy = exact range,
      colors intact, zero standing cost (typing/scroll unchanged).
- [ ] 2.6 Docs: MOBILE.md selection section; memory update; archive change.
