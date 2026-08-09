# Tasks — hq-signal-ergonomics

Status: IN PROGRESS — sections 1, 2, 3 and 5 landed; 4, 6, 7 remain.

## 1. The grade projection (one source, no new judgment)

- [x] 1.1 Define the class/severity → grade (decision/attention/ledger) mapping in one
      place next to the wake-class table (`internal/hqwake`), covering every declared
      class; conformance test: no class without a grade.
- [x] 1.2 Glyphs chosen against the `»`/`│` bar (POSIX locale, CJK fonts, agent composer):
      `◆` decision · `▸` attention · `·` ledger — Geometric Shapes and Latin-1, the blocks
      the grammar already uses; no emoji (variation-selector / presentation baggage).
      Pinned by a test that rejects anything above U+2BFF. Documented in `docs/cli.md`
      beside the signal grammar and in the playbook.

## 2. Wake lines carry the grade

- [x] 2.1 Extend `hqwake.Line` with the fixed-position grade glyph; update the fixture
      tests (extend, don't fork the grammar).
- [x] 2.2 `docs/cli.md` class table gains the grade column (the docs-conformance check
      requires class docs and playbook to stay in step).

## 3. HQ's register and print gate (playbook)

- [x] 3.1 Playbook: the ⟣ vocabulary maps onto the three grades; ledger-grade content is
      board/ledger-only, never printed; grade glyph leads the signal line.
- [x] 3.2 Bump `hqPlaybookVersion` (repo rule: any `hqInstructions` change bumps it).

> **Found while implementing (both fixed, both now regression-tested):** two production
> detectors matched the wake line by the literal prefix `» gtmux·`, which the grade glyph
> splits. `transcript.gtmuxEchoPrefixes` — HQ's own line echoed back would have read as a
> real user goal (a `goal-changed` wake about itself, at instruction severity). And
> `hqwake.BatchID` — the delivery ack would stop recognising its own batch, so every wake
> would re-send until the channel declared itself degraded. Both now match the SIGIL alone,
> which is robust to the grammar growing again.

## 4. Pending-decision standing view

- [ ] 4.1 Attention ledger: first-class "awaiting-commander" disposition (additive field
      semantics; legacy entries unaffected).
- [ ] 4.2 `gtmux tasks --pending` (or the surface decided at implementation): the standing
      view, stable ordering, en+zh; updates only on state change.
- [ ] 4.3 Tests: entering/leaving the pending set; legacy ledger loads.

## 5. Delta-only tick briefs

- [x] 5.1 Playbook: a tick brief names only changes since the last brief; the unchanged
      plate is a one-clause pointer to the pending view (covered by the same version
      bump as 3.2).

## 6. Color on gtmux-owned surfaces

- [ ] 6.1 `events --follow` / `tasks`: grade-colored rendering, tty-gated, `NO_COLOR`
      respected; non-tty byte-identical to today.
- [ ] 6.2 Tests for the gating (tty vs pipe vs NO_COLOR).

## 7. Consistency (per the repo rule)

- [ ] 7.1 Sync deltas into `openspec/specs/{hq-wake-protocol,supervisor-agent,hq-attention-system}/spec.md`.
- [ ] 7.2 `docs/cli.md` (`gtmux tasks` section + class table), CLAUDE.md lines citing the
      signal format if wording drifts; `api/contract.md` untouched (no HTTP change).
- [ ] 7.3 Archive this change once implemented.
