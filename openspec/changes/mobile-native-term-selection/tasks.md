# Tasks — mobile-native-term-selection

## Stage 1 — uniform grid (JS)
- [ ] 1.1 Explicit uniform lineHeight for terminal rows (fontSize-derived; both
      layers on Android keep parity — its overlay Text gets the same value).
- [ ] 1.2 Cell-arithmetic pre-wrap: logical line → visual rows (CJK=2 cells,
      conservative row capacity from measured/derived cell width; never lets a
      row native-wrap). Rows carry their source line + char offsets.
- [ ] 1.3 Extend termLineCache: cache parsed spans AND wrapped rows per raw
      line (+ width/fontSize in the cache signature). Parity test: joined rows
      == logical line text; no row exceeds capacity.
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
