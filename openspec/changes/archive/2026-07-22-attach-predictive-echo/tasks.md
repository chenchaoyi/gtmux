# Tasks

## 1. `OpCursor` frame (plumbing)
- [x] 1.1 `internal/connect/frame.go`: `OpCursor byte = 'c'` + `EncodeCursor(x,y,alt)` /
      `DecodeCursor(payload)` (JSON `{x,y,alt}`), unit-tested round-trip.
- [x] 1.2 `internal/server/attach.go`: sample tmux (`#{cursor_x},#{cursor_y},#{alternate_on}`)
      on a small cadence + after each output batch, send `OpCursor` (own goroutine,
      non-blocking, coalesce/skip unchanged).
- [x] 1.3 `internal/connect/attach.go`: read `OpCursor`, track `serverCursor` (no prediction
      yet; optional debug readout under a debug env).

## 2. The predictor (pure core, tested first)
- [x] 2.1 A pure predictor: state (predicted string, epoch, rtt), `onKey` (printable/backspace
      vs epoch-ending keys), `reconcile(serverCursor)` (drop confirmed prefix / clear on
      move), `predicting()` gate (rtt threshold + !alt). Unit-tested with no terminal.
- [x] 2.2 Feed it: forward every keystroke raw, then predict; draw predicted chars in the
      unconfirmed (underline+dim SGR) style; backspace rubs out locally.

## 3. Reconcile + erase on the terminal
- [x] 3.1 Before writing an `OpOutput` batch to stdout, erase the outstanding predicted tail
      (cursor back + `ESC[K`), then write the real bytes.
- [x] 3.2 On `OpCursor`, drop the confirmed prefix / re-seed; on any state-changing key,
      `endEpoch()` (erase + clear + pause).

## 4. Gating + flag
- [x] 4.1 `--predict` flag (+ optional config) — OFF by default; measure the send→confirm
      RTT into the EWMA; predict only above threshold, only when `!alt`.

## 5. Spec + docs
- [x] 5.1 Sync the `remote-terminal-client` spec (this change's delta) on archive.
- [x] 5.2 `docs/cli.md` `gtmux attach`: document `--predict` (experimental, adaptive,
      underlined-unconfirmed).

## 6. Verify
- [x] 6.1 `CGO_ENABLED=0 go build ./cmd/gtmux` + `make check` green.
- [ ] 6.2 Manual: high-latency shell prompt (predict+confirm), vim (no predict), a mispredict
      (erased, not left). **PARTIAL — deliberately left unchecked.** VERIFIED on a FAST
      link (localhost): attach connects, the adaptive gate draws NO predictions, and the
      typed text lands once with no doubling/corruption. NOT verified on a SLOW link:
      driving a raw-mode attach from inside tmux nests tmux and garbles both the render
      and the capture, so the slow-path predict/erase (and the vim + mispredict cases)
      still need a real terminal. Shipped opt-in (`--predict`, default OFF) partly for
      this reason. Known residual risk: a predicted run crossing a right-margin WRAP —
      the back-N-columns erase can land on the wrong row; Ctrl-L / reattach recovers.
