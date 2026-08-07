# Tasks — hq-signal-ergonomics

Status: PROPOSED (not started).

## 1. The grade projection (one source, no new judgment)

- [ ] 1.1 Define the class/severity → grade (decision/attention/ledger) mapping in one
      place next to the wake-class table (`internal/hqwake`), covering every declared
      class; conformance test: no class without a grade.
- [ ] 1.2 Choose the three glyphs for encoding robustness (survive POSIX locale, CJK
      fonts, and the agent composer) — same bar the `»`/`│` choice met; document the
      choice where the signal grammar is documented.

## 2. Wake lines carry the grade

- [ ] 2.1 Extend `hqwake.Line` with the fixed-position grade glyph; update the fixture
      tests (extend, don't fork the grammar).
- [ ] 2.2 `docs/cli.md` class table gains the grade column (the docs-conformance check
      requires class docs and playbook to stay in step).

## 3. HQ's register and print gate (playbook)

- [ ] 3.1 Playbook: the ⟣ vocabulary maps onto the three grades; ledger-grade content is
      board/ledger-only, never printed; grade glyph leads the signal line.
- [ ] 3.2 Bump `hqPlaybookVersion` (repo rule: any `hqInstructions` change bumps it).

## 4. Pending-decision standing view

- [ ] 4.1 Attention ledger: first-class "awaiting-commander" disposition (additive field
      semantics; legacy entries unaffected).
- [ ] 4.2 `gtmux tasks --pending` (or the surface decided at implementation): the standing
      view, stable ordering, en+zh; updates only on state change.
- [ ] 4.3 Tests: entering/leaving the pending set; legacy ledger loads.

## 5. Delta-only tick briefs

- [ ] 5.1 Playbook: a tick brief names only changes since the last brief; the unchanged
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
