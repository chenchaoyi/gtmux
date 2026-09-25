# Tasks

## The advice ledger
- [x] `internal/advice`: append-only JSONL in HQ's home, give + mark, folded at read
- [x] an outcome is a new record, never an edit, so a reversal keeps its history
- [x] tally: given / taken / declined / open / overtaken, with the rate computed only
      from the two that are verdicts
- [x] a torn line does not take the rest of the ledger down with it
- [x] `gtmux advice` with its usage in both languages, writes gated to the HQ home,
      reads working anywhere
- [x] `act.advice` in the diag catalog, emitted on both write paths
- [x] tests, each confirmed by re-introducing its defect

## The charter
- [x] 13: say back what you heard before acting on it, with the three things to raise
- [x] 14: advise while it can still change — four ways, and say the problem directly
- [x] 15: holding is a third disposition, always spoken, with the three that are never
      held and the refusal to disguise one
- [x] 16: keep the counsel ledger
- [x] both language halves, `hqPlaybookVersion` 49

## The writing rule
- [x] `say-the-problem` in the plain-language table: the commander's standard governs,
      and a verdict does not precede its facts

## Docs
- [x] CLAUDE.md command list, helpdata entry, `docs/cli.md` + `.zh.md` section
- [x] `make docs-fix` for the act catalog region

## Verify
- [x] `make check` + `check-design.sh` by exit code
- [x] sync-specs + archive
