## Context

`@fission-ai/openspec@2.2.0` introduces two changes relevant to `gtmux`:
1. `openspec init --scope project --tools agents,claude` installs six skills (`openspec`, `openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `openspec-explore`, `openspec-sync-specs`), drops legacy `.claude/commands/opsx/`, and by default creates symlinks in `.claude/skills/* -> ../../.agents/skills/*` — which `scripts/ci.py` forbids in git.
2. `openspec validate --specs --strict` adds placeholder-Purpose checks (caught 4 specs, fixed in this branch) and an advisory `Requirement text is very long (>500 characters)` warning that fires on 576 historical requirements across 40 specs, while `openspec validate <change> --strict` enforces `<=500` chars on newly added requirements without penalizing unmodified main specs.

## Decisions

1. **Materialize regular files in `.claude/skills/`**: Run `openspec init --scope project --tools agents --no-animation` and copy the 6 skill directories as regular files into `.claude/skills/` (deleting retired `.claude/commands/opsx/`), enforced by `scripts/workflow_guard.py`.
2. **Strict structural/semantic enforcement in `scripts/openspec_check.py`**: Run `@fission-ai/openspec@2.2.0 validate --specs --strict --json`; treat every `ERROR` and every non-length `WARNING` (missing `SHALL`/`MUST`, missing `WHEN`/`THEN`, missing scenarios, placeholder/brief `Purpose`) as `"invalid"` while allowing historical `>500`-character prose advisories in `openspec/specs/` and verifying exact exit codes (`0` when raw_failed==0, `1` when raw_failed>0).
