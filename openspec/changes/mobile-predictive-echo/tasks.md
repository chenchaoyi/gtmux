# Tasks

## 1. The predictor (pure, tested first)
- [ ] 1.1 A `predictiveEcho` module: state (outstanding predictions, local cursor, RTT
      EWMA), `onKey(char)` (printable/backspace → prediction; epoch-ending keys → clear),
      `reconcile(screen, serverCursor)` (drop confirmed/contradicted, re-seed cursor),
      `shouldPredict()` (RTT threshold + cursor.visible gate).
- [ ] 1.2 Unit tests (jest): advance/underline, backspace, reconcile confirm vs contradict,
      epoch reset clears, gating off below threshold / in alt-screen.

## 2. Render the overlay
- [ ] 2.1 `term.ts` / `NativeTerm.tsx`: draw outstanding predicted cells at their
      positions, underlined/dimmed, over the authoritative screen (reuse `cursorSpans`
      positioning; new style for "unconfirmed").

## 3. Wire into the input + capture loop
- [ ] 3.1 DetailScreen/Composer: feed each keystroke to the predictor before/as it POSTs
      `/api/send`; measure the send→screen round-trip into the RTT EWMA.
- [ ] 3.2 Reconcile on the `/api/send` response screen AND on each capture poll; seed the
      local cursor from `PaneResponse.cursor`.
- [ ] 3.3 Epoch-end on Enter/ESC/arrows/Ctrl-C/Tab and the 1/2/3 approval taps; no
      predictor for guest read-only panes or `cursor.visible=false`.

## 4. Spec + docs
- [ ] 4.1 Sync the `mobile-pane-renderer` spec (this change's delta) on archive.
- [ ] 4.2 MOBILE.md §4: note predictive local echo (adaptive, underlined-unconfirmed).

## 5. Verify
- [ ] 5.1 `cd mobileapp && npm run check` green (tsc + eslint + jest).
