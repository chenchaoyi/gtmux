## 1. Version stamp + parsing

- [x] 1.1 Add a `hqPlaybookVersion` constant to `internal/app/hq.go` and prepend a
  machine-parseable managed-marker line to the generated `hqInstructions`
  (`<!-- gtmux-hq-playbook v<N> · managed by gtmux — DO NOT EDIT; put your own
  instructions in LOCAL.md -->`).
- [x] 1.2 `parsePlaybookVersion(body string) int` — read the version from an AGENTS.md's
  marker (no marker → 0). Pure, unit-tested (marker present / absent / malformed).

## 2. LOCAL.md personalization file

- [x] 2.1 Add `hqLocalPath()` + a `LOCAL.md` template ("gtmux never overwrites this;
  your instructions here override/extend the managed playbook"). End the generated
  AGENTS.md body with an `@LOCAL.md` import.
- [x] 2.2 Seed `LOCAL.md` once if absent (seed-if-absent, never overwrite); reseed only
  the template when a managed AGENTS.md exists but LOCAL.md was hand-deleted (so the
  import resolves). Unit-test never-overwrite.

## 3. Upgrade-on-version-bump in seedHQHome

- [x] 3.1 Rework `seedHQHome()`: fresh home → write versioned AGENTS.md + CLAUDE.md import
  + LOCAL.md. Existing managed AGENTS.md with installed < shipped → back up to
  `AGENTS.md.bak-v<old>`, regenerate at the shipped version, signal `upgraded(old→new)`.
  Installed == shipped → no-op. Keep the legacy full-CLAUDE.md handling.
- [x] 3.2 Legacy migration: an unversioned AGENTS.md (v0) with shipped ≥ 1 → same
  backup+regenerate, with a migration flag so the notice points to LOCAL.md.
- [x] 3.3 `gtmux hq` prints the one-line upgrade/migration notice (bilingual) when
  seedHQHome reports an upgrade; silent when idempotent.
- [x] 3.4 Unit-test: fresh seed writes marker+LOCAL.md; newer version upgrades (backup
  exists, new marker, LOCAL.md untouched); equal version is a no-op; legacy v0 migrates;
  board.md/knowledge left untouched across an upgrade.

## 4. Policy-layout warnings

- [x] 4.1 Update `hqPolicyWarning`/`isClaudePointer` for the managed model (the legacy
  full-CLAUDE.md + dangling-import warnings still apply). Adjust `hq_test.go`.

## 5. Docs, memory, gate

- [x] 5.1 CLAUDE.md: note the seed lifecycle is now version-tracked — a playbook change
  MUST bump `hqPlaybookVersion` to ship to existing homes; personal edits go in LOCAL.md.
- [x] 5.2 Update the memory notes that flagged "seed-once won't auto-update"
  ([[hq-attention-system]], [[hq-nudge-hardening-followups]], [[hq-chief-of-staff]]) to
  point at the new versioned mechanism; add a `versioned-hq-playbook` memory.
- [ ] 5.3 `make check` green; `openspec validate versioned-hq-playbook --strict` valid.
  Sync-specs + archive when merged.
