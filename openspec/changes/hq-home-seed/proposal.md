# Single-source HQ home seeding — no zombie policy doc

## Why

`gtmux hq`'s `seedHQHome` writes AGENTS.md into ANY home lacking it — including one
that already holds a full, user-edited CLAUDE.md (the legacy layout, from before the
AGENTS.md convention). Result on a Claude HQ: TWO policy docs in the home —
CLAUDE.md (what Claude actually reads) and a freshly-seeded AGENTS.md (10 KB default
template Claude ignores). The AGENTS.md is a **zombie**: it never matches the edited
CLAUDE.md, misleads anyone who opens the home, and — worse — deleting it doesn't
stick, since the next `gtmux hq` re-seeds it. Observed live: an AGENTS.md appeared
beside a curated CLAUDE.md and immediately drifted.

The spec even enshrines the bug: "each file is seeded only when absent — an older
full CLAUDE.md is never clobbered" describes exactly the behavior that drops the
zombie.

## What Changes

Seed exactly ONE authoritative policy file — never a second — with a single source of
truth (user-decided):

- **Single source:** AGENTS.md is the canonical FULL playbook; CLAUDE.md is a one-line
  `@AGENTS.md` import so Claude reads the SAME content (no two-doc drift).
- **Fresh home** (no policy file) → seed AGENTS.md (full) + CLAUDE.md (import).
- **A home that already has a policy file is "already seeded"** → never add a second
  FULL doc. In particular a **legacy full CLAUDE.md is authoritative** → do NOT drop a
  zombie AGENTS.md beside it. No policy file is ever overwritten (idempotent, respects
  user edits).
- **AGENTS.md-only** (e.g. a prior Codex seed, Claude entry missing) → add ONLY the
  cheap CLAUDE.md `@AGENTS.md` import, never a full copy.
- **Warn, don't silently live with it:** `gtmux hq` surfaces a redundant layout (a full
  CLAUDE.md alongside AGENTS.md) or a broken one (a CLAUDE.md `@AGENTS.md` import with
  AGENTS.md missing).

Out of scope (handled as a live-machine cleanup, not code): the existing zombie
AGENTS.md on the user's machine is merged into their CLAUDE.md and deleted.

## Impact

- Spec: `supervisor-agent` "Launchable supervisor session" (seed behavior + scenarios).
- Code: `internal/app/hq.go` (`seedHQHome`, new `isClaudePointer` / `hqPolicyWarning`,
  the `cmdHQ` warn print).
- Tests: `internal/app/hq_test.go`.
- No contract change — the home layout is internal; the change only stops creating a
  redundant file and adds a warning.
