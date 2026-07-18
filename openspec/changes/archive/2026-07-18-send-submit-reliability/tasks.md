# Tasks: send-submit-reliability

## 1. Full-payload fingerprint primitive

- [x] 1.1 Add `NormalizeTail(s string) string` (last `headRunes` runes of the
  space-normalized text) and `ContainsTail(haystack, needle string) bool` in
  `internal/dispatch/head.go`, mirroring `NormalizeHead` / `ContainsHead`.
- [x] 1.2 Unit tests in `head_test.go`: tail of a long payload, head==tail for
  short payloads, whitespace-re-wrap tolerance.

## 2. Full-match draft predicate

- [x] 2.1 Change `draftHasDelivery(draft, text)` in `internal/dispatch/deliver.go`
  to require `ContainsHead && ContainsTail` (or `looksCollapsedPaste`), so a
  head-only draft is a fragment.
- [x] 2.2 Confirm `confirmPaste` / `pasteWithGuard` now wait out a slow tail (the
  fragment verdict only after the full settle window) — no control-flow change,
  just the stronger predicate.

## 3. Don't blindly re-Enter

- [x] 3.1 In `Deliver`'s verify loop, ensure the swallowed-Enter branch fires only
  on the full-match `inDraft` (now guaranteed by 2.1) and add the explicit comment
  that a re-Enter requires the draft to still hold the full target.

## 4. Pre-submit confirmation on the unverified paths

- [x] 4.1 Factor a `PasteAndSubmit(io, opts, text) (confirmed bool)` helper in
  `internal/dispatch` that pastes (with the existing fragment guard), waits up to
  the settle window for the FULL draft, then sends Enter once.
- [x] 4.2 Route `app.sendToPane` (`POST /api/send`) text-with-Enter through it
  (build the same injected `IO` as the verified path, minus the events/landed
  loop). Keep `--key` and text-only (`enter:false`) unchanged.
- [x] 4.3 Route `cmdSend`'s plain path (`--no-verify` / non-`--no-enter`) through
  the same helper.

## 5. Tests

- [x] 5.1 `deliver_test.go`: a payload whose tail renders a frame after the head is
  NOT submitted until the tail arrives (regression for truncation-as-landed).
- [x] 5.2 `deliver_test.go`: a draft that holds only the head is reported as a
  fragment, not landed.
- [x] 5.3 `deliver_test.go`: re-Enter is not sent once the draft is empty / no
  longer matches.
- [x] 5.4 A test for `PasteAndSubmit` confirming Enter is withheld until the full
  draft is present, and sent best-effort after the window.
- [x] 5.5 Update any existing dispatch fixtures that asserted head-only landing to
  carry a tail.

## 6. Spec / docs / gate

- [x] 6.1 `make check` green (gofmt + vet + staticcheck + `go test -race`).
- [x] 6.2 `openspec validate send-submit-reliability --strict` passes.
- [x] 6.3 Sync the deltas into `openspec/specs/agent-dispatch/spec.md` and archive
  the change (same PR or the next), keeping these checkboxes truthful.
- [x] 6.4 Confirm no doc cites head-only landing (`docs/cli.md`, `api/contract.md`
  unaffected — no wire change; grep to be sure).
