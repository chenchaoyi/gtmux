## Why

The HQ playbook (`~/.config/gtmux/hq/AGENTS.md`) is seeded ONCE and never overwritten,
so a `gtmux update` ships a new charter that never reaches an existing HQ — the recurring
"seed-once" footgun (hit by supervisor-mvp, hq-chief-of-staff, hq-nudge-hardening, and now
the attention system). The root cause is that ONE file conflates two concerns that want
opposite lifecycles: the **playbook** (gtmux's behavior charter — should track the gtmux
version) and the user's **personalization** (should persist untouched).

Split them: make the playbook a **version-tracked, gtmux-owned** file that upgrades with
the binary, and move user personalization to a **separate, never-overwritten** file the
playbook imports. Accumulated memory (`notes/board.md`, `knowledge/*`) is already separate
and stays untouched.

## What Changes

- **BREAKING (seed contract):** `AGENTS.md` becomes **gtmux-owned and version-tracked**,
  not a user-edit target. It carries a `playbookVersion` stamp; `gtmux hq` regenerates it
  when the shipped version is newer (so `gtmux update` + the next `gtmux hq` auto-upgrades
  the charter). The previous file is backed up to `AGENTS.md.bak-<oldver>` first, and the
  upgrade prints a one-line notice.
- **New `LOCAL.md`** in the hq home: a seed-once, NEVER-overwritten file for the user's
  own instructions/preferences, `@`-imported by the generated `AGENTS.md` (and thus
  reaching Claude via the existing `CLAUDE.md → @AGENTS.md` chain). This is where personal
  edits go now.
- **Legacy migration:** an existing `AGENTS.md` with no version stamp (a user may have
  edited it under the old contract) is treated as version 0 — on the first upgrade it is
  backed up and the notice points the user to `LOCAL.md` for any personal edits to carry
  over. A full standalone `CLAUDE.md` (pre-AGENTS.md convention) keeps its existing
  authoritative treatment; versioning applies to the AGENTS.md playbook.
- **Unchanged:** `notes/board.md` (situation board) and `knowledge/*` (KB) stay
  seed-if-absent and are never touched by an upgrade — the durable memory the user meant.

## Non-goals

- Not touching the ATTENTION-SYSTEM behavior itself — only how the playbook FILE is
  delivered/upgraded. (The phase-④ seed content becomes an upgrade rather than a manual
  re-seed.)
- Not a general config-migration framework — just the hq playbook file + a local override.
- Not merging user edits INTO the regenerated playbook (approach A: clean ownership split,
  personalization lives in `LOCAL.md`, not marker-merged into AGENTS.md).

## Capabilities

### Modified Capabilities
- `supervisor-agent`: the seed lifecycle changes from "generate once, never overwrite any
  policy file" to "the AGENTS.md PLAYBOOK is gtmux-owned and VERSION-UPGRADED (backing up
  the prior on change), while user PERSONALIZATION lives in a seed-once `LOCAL.md` the
  playbook imports and gtmux never overwrites; the situation board and knowledge base stay
  untouched."

## Impact

- **Code:** `internal/app/hq.go` — a `hqPlaybookVersion` constant + a version stamp in the
  generated AGENTS.md; `seedHQHome()` gains upgrade-on-version-bump (backup + regenerate)
  and a `LOCAL.md` seed-once; the `hqPolicyWarning` layout checks adjust; `gtmux hq` prints
  the upgrade notice. Tests in `hq_test.go`.
- **Files (hq home):** `AGENTS.md` (now managed), `AGENTS.md.bak-<ver>` (backups),
  `LOCAL.md` (new, user-owned). `CLAUDE.md`, `notes/board.md`, `knowledge/*` unchanged.
- **Contract:** the supervisor-agent spec's seed requirement is rewritten (a deliberate
  behavior change); the bundle/state contracts and `agents --json` are untouched.
- **Docs/memory:** CLAUDE.md seed-lifecycle note; a memory recording the new contract so
  future seed changes just bump `hqPlaybookVersion`.
