# transcript-render-bounds — tasks

## 1. Server: bound the payload by bytes

- [x] 1.1 Keep the newest turns within a byte budget; always keep at least one turn
- [x] 1.2 Report the dropped-turn count to the client
- [x] 1.3 Tests: long history truncates to its tail · short history untouched · a single oversized turn is still served · dropped count is accurate

## 2. Mobile: render a window

- [x] 2.1 Render only the newest N turns; "load earlier" extends the window
- [x] 2.2 Disclose hidden turns (windowed + server-dropped) in one honest line
- [x] 2.3 Tests for the window/disclosure logic

## 3. Docs + gates

- [x] 3.1 `api/contract.md` documents the bound + the dropped-turn signal
- [x] 3.2 `make check` · `mobileapp npm run check` · `check-design.sh` green
- [x] 3.3 openspec validate
- [x] 3.4 sync-specs + archive
