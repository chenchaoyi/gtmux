## Context

`seedHQHome()` (internal/app/hq.go) today:
- writes `AGENTS.md` (the full playbook `hqInstructions`) + `CLAUDE.md` (`@AGENTS.md`
  import) only when absent, and NEVER overwrites either — to protect user edits + any
  accumulated content;
- seeds `notes/board.md` and `knowledge/*` only if absent (already separate, already
  untouched on re-run).

The result: a shipped charter change never reaches an existing home. The supervisor-agent
spec pins this as "SHALL NOT overwrite any policy file." Claude loads `CLAUDE.md`, which
`@`-imports `AGENTS.md`; Claude resolves nested `@`-imports, so an `AGENTS.md` that imports
`LOCAL.md` pulls the user's overrides through the same chain.

## Goals / Non-Goals

**Goals:**
- A shipped playbook change reaches an existing HQ automatically on the next `gtmux hq`
  after `gtmux update` — no manual re-seed.
- User personalization + memory are NEVER clobbered by an upgrade.
- The upgrade is safe (backup before overwrite) and visible (a one-line notice).

**Non-Goals:**
- No marker-based merge of user edits into the generated playbook (approach A: clean
  ownership split).
- No change to the attention-system behavior, the board, or the knowledge base.

## Decisions

### D1 — Version stamp lives in the generated AGENTS.md header

The generated `AGENTS.md` opens with a machine-parseable managed-marker line, e.g.:

```
<!-- gtmux-hq-playbook v3 · managed by gtmux — DO NOT EDIT; put your own instructions in LOCAL.md -->
```

`gtmux hq` parses the installed version from this line and compares it to the shipped
`hqPlaybookVersion` constant. A file with NO marker (a legacy/user-edited AGENTS.md) parses
as version 0. This keeps the version WITH the artifact (no separate sidecar to drift), and
the marker doubles as the "don't edit me" signal.

*Alternative considered:* a sidecar `hq/.playbook-version`. Rejected — a second file that
can drift from AGENTS.md; the in-file marker can't.

### D2 — Upgrade = backup then regenerate, only when shipped > installed

`seedHQHome()`:
- Fresh home (no AGENTS.md): write the current playbook (with the vN marker) + `CLAUDE.md`
  + `LOCAL.md` (empty template) — as today, plus the marker and LOCAL.md.
- Existing managed AGENTS.md, installed < shipped: **back up** to `AGENTS.md.bak-v<old>`,
  **regenerate** at vN, return an `upgraded (old→new)` signal so `gtmux hq` prints a notice.
- Installed == shipped: no-op (idempotent).
- Legacy AGENTS.md (no marker → v0) while shipped ≥ 1: same backup+regenerate path, but the
  notice explicitly says "your previous playbook is backed up at …; move any personal edits
  into LOCAL.md." This is the one-time migration for homes edited under the old contract.

Downgrades never happen (a dev binary older than the home just no-ops). The backup is
keep-all (`-v<old>` suffix), so no upgrade destroys prior content irreversibly.

### D3 — LOCAL.md is the seed-once personalization file

`AGENTS.md`'s generated body ends with `@LOCAL.md` (an import Claude/Codex resolve).
`LOCAL.md` is written ONCE with a short template ("# Your HQ instructions — gtmux never
overwrites this. Anything here overrides/extends the managed playbook.") and NEVER
regenerated. Since it is imported LAST, its content can override earlier playbook guidance
by convention. board.md/knowledge stay exactly as they are (separate, seed-if-absent).

### D4 — hqPolicyWarning adjusts to the managed model

The current warnings (redundant full CLAUDE.md + AGENTS.md; dangling `@AGENTS.md`) still
apply for the legacy full-CLAUDE.md case. Add: if a managed AGENTS.md marker is present but
`LOCAL.md` is missing (e.g. a hand-deleted LOCAL.md), reseed just the LOCAL.md template
(cheap, seed-if-absent) rather than warn — the import must resolve.

## Risks / Trade-offs

- **[A user who edited AGENTS.md under the old contract loses their edits on first upgrade]**
  → Mitigated by the backup (`AGENTS.md.bak-v0`) + an explicit migration notice pointing to
  LOCAL.md. Nothing is destroyed; the user copies their edits into LOCAL.md once.
- **[Regenerating on every version bump could surprise a user watching the file]** → The
  managed-marker line says DO NOT EDIT and names LOCAL.md; the notice fires only on an
  actual version change, and backups accumulate so it's always recoverable.
- **[Nested @-import depth]** → CLAUDE.md→AGENTS.md→LOCAL.md is two levels; Claude Code
  resolves nested imports. Codex/Cursor read AGENTS.md directly, also resolving its import.
- **[Version constant discipline]** → Future playbook edits MUST bump `hqPlaybookVersion`
  or they won't ship to existing homes. Called out in CLAUDE.md + a memory so it becomes the
  standard ritual (the whole point of this change).

## Migration Plan

One PR. On rollout, the first `gtmux hq` on an existing home performs the one-time
legacy→managed migration (backup + regenerate + LOCAL.md + notice). No data is destroyed;
the board and KB are untouched. Rollback: revert restores seed-once behavior; a home already
migrated keeps its managed AGENTS.md + LOCAL.md harmlessly (both still valid files).

## Open Questions

- Should `gtmux hq` DIFF the old vs new playbook in the notice, or just name the backup?
  (Leaning: name the backup + a one-line "what changed" summary keyed to the version; a full
  diff is noise on a phone.)
- Prune old `AGENTS.md.bak-v*` backups beyond the last few? (Leaning: keep all for now —
  they're tiny; revisit if they accumulate.)
