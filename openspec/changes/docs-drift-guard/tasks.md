# Tasks: docs-drift-guard

Phase A only. Phase B (generating the class table from a registry) is deliberately NOT
scheduled here — see the proposal; it is a decision to take after A proves the machinery.

## 1. Rendered examples

- [ ] 1.1 Doc-example registry in Go beside the builders: id → the exact call that
      produces the line (`wake-waiting`, `wake-done` to start — the two `docs/cli.md`
      already carries).
- [ ] 1.2 Marker parsing (`<!-- gtmux:rendered <id> -->` … `<!-- /gtmux:rendered -->`)
      over the docs tree + a fixture test comparing each region to its builder, failing
      with BOTH lines printed. Tests: matching region passes; a mutated region fails; a
      registered id with no region fails; an unmarked example is ignored.
- [ ] 1.3 `make docs-fix` rewrites the regions from the code; CI only reads. Test the
      rewrite is idempotent and touches nothing outside the markers.
- [ ] 1.4 Mark the two existing `docs/cli.md` wake examples (they are already
      byte-accurate — #478 verified them by hand with a throwaway probe; this is that
      probe made permanent).

## 2. Enumeration + denylist (check-design.sh, beside §6)

- [ ] 2.1 Every `hqwake.Class*` must appear in `docs/cli.md`'s class table AND in
      `hqInstructions`; `DOC_HIDDEN` allowlist for deliberate omissions. Verify it goes
      RED by temporarily adding a class (then reverting) — a check nobody has seen fail is
      not a check.
- [ ] 2.2 Retired-token denylist with a `# retired by <change>` column: the `[gtmux] `
      wake prefix, `internal/menubar/`, "`important` = the attention stream". Verify RED
      the same way.

## 3. Rules + consistency

- [ ] 3.1 CLAUDE.md: the docs-conformance rule next to the spec⇄code⇄test rule, INCLUDING
      the boundary — a green gate is not a reviewed doc; prose-to-behavior stays a
      reviewer's judgment.
- [ ] 3.2 `make check` + `scripts/check-design.sh` + `openspec validate --specs --strict`
      green.
- [ ] 3.3 Sync the delta into `openspec/specs/` and archive this change once merged.
