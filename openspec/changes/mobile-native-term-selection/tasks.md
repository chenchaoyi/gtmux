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
- [x] 2.1 `TermSelectionView.swift`: UITextInput read-only subset over
      rows+rowH+font props; CTLine cache; DocumentPosition/Range types
      (TermTextPosition/TermTextRange over UTF-16 offsets into rows joined
      '\n'; row = y ÷ rowHeight arithmetic, in-row x via cached CTLines).
- [x] 2.2 UITextInteraction + UIEditMenuInteraction wiring (long-press select —
      recognized on the enclosing scroll view so the pass-through overlay never
      blocks links/scroll — drag extends, handles both directions, Copy → 
      clipboard via the iOS 16 edit menu, en/zh label; tap-away clears).
- [x] 2.3 RCTViewManager registration (`TermSelectionViewManager` → legacy
      interop) + `<TermSelection>` mounted absoluteFill over the row stack in
      layerWrap (iOS only, pointerEvents box-none); onSelectionActive freezes
      polls (flushPending refuses while held); native clears on text change.
- [x] 2.4 Delete the JS select-sheet path; remove now-dead styles/state
      (selText/selRange/openSelectAt/veil/Done chip + block long-press).
- [x] 2.5a SIMULATOR verification (iPhone 17 Pro / iOS 26.5, Appium-driven,
      screenshots /tmp/sel-*.png): band + both lollipop handles AT the pressed
      word (demo tail AND deep live CJK history after scrolling up — no jump);
      handle drags in BOTH directions (start up-left across rows, end down);
      Copy byte-exact (multi-row ASCII across wrap+empty rows; CJK "定案");
      band rides the scroll glued to its glyphs; tap-away clears; composer
      typing (incl. CJK) + scrolling unaffected with the overlay mounted.
      Three sim-found bugs fixed in the same pass: selection visuals need an
      explicit UITextSelectionDisplayInteraction (and UITextInteraction must
      NOT be attached — its internal gesture cleared selections mid-drag), a
      slow handle drag lost to the long-press recognizer (fixed by
      touch-reception routing), processColor ARGB decoded as RGBA by the
      interop layer (tint now a hex-string prop).
- [ ] 2.5b DEVICE verification (remainder that a sim can't honestly cover):
      loupe visibility during drags (implemented via UITextLoupeSession;
      mid-gesture sim screenshot inconclusive), real-finger feel of the handle
      grab zones, typing/scroll smoothness under a streaming pane on hardware,
      link taps still opening with the overlay mounted (pass-through verified
      for scroll/long-press/composer on sim, not for a link tap).
- [ ] 2.6 Docs: MOBILE.md selection section; memory update; archive change.
