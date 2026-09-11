# Tasks — hq-transcript-mining

- [x] 1. `internal/mine`: Claude Code log reader (offset-resumable, sidechain/meta/compaction skipped, typed prompts via `transcript.ClassifyUserPrompt`)
- [x] 2. `internal/mine`: machine-injection subtraction against the audit journal (`send` / `wake-delivered` heads)
- [x] 3. `internal/mine`: correction lexicon (zh+en) + punctuation emphasis; candidate = human line + assistant tail + ids
- [x] 4. `internal/mine`: error signature normalizer + cross-session tally, test-runner summaries excluded
- [x] 5. `internal/mine`: ledger (sources / emitted / errors / passes), never emits twice, re-reads a shrunken file from 0
- [x] 6. `internal/hq`: spool fields (`source`, `context`, `session`, `project`, `count`), `capture --list` renders them
- [x] 7. `internal/hq`: `mineSensor` in the slow tick, `hqWake.mineIntervalHours`
- [x] 8. `internal/hq`: `gtmux knowledge mine [--dry-run] [--since] [--json] [--status]`
- [x] 9. Playbook v37 (en + zh): what a transcript candidate is and how to file it
- [x] 10. Docs: `docs/cli.md` + `docs/cli.zh.md` (`knowledge mine`, the spool line), `docs/design/knowledge-layers.md` (the miner beside the candidate pool), CLAUDE.md pointer
- [x] 11. Tests on synthetic logs only: each subtraction verified by removing it; ledger idempotence; sensor gate
- [x] 12. Run on this machine, report counts; spec delta synced; change archived
