# Tasks — slim-attention-ledger

Status: IMPLEMENTED (2026-08-17). Deletion-only code; the teaching fixes rode with it.

- [x] 1. dispatch: delete Promote/SetPriority/MarkSurfaced/ArchiveTask/
     ListArchived + the archive dir; drop Tier/Priority/Surfaced/SurfacedAt/
     Archived/ArchivedAt from Task (legacy files still load — unknown JSON
     fields are ignored); delete the dead `!t.Archived` filter condition;
     tests for the deleted verbs go with them, disposition/pending coverage
     stays green.
- [x] 2. taskscmd: taskJSON/rowFor shrink, gatherArchivedTasks and the
     "archived" status render go, verboseTail keeps only disposition, help
     text says what --verbose now adds.
- [x] 3. Playbook: the QUIET-item bullet loses "can be PROMOTED later";
     self-check's ledger duty becomes settling stale pending entries.
     `hqPlaybookVersion` 29 → 30. docs/cli.md self-check row + CLAUDE.md
     ledger summary follow.
- [x] 4. Spec deltas: hq-attention-system `Attention ledger` REMOVED +
     `Pending-decision standing view` MODIFIED; supervisor-agent self-check
     MODIFIED (drops "ledger archival" and the leftover "feed health").
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
