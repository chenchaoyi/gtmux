# Tasks

## 1. Resolve against the transcript store
- [x] 1.1 `internal/resume/resolve.go`: locate `~/.claude/projects/*/<sid>.jsonl`, read the
      transcript's `cwd`, expose `Resolve(Record) (Record, alive bool)`.
- [x] 1.2 Call it in `agent_resume.go` before `resume.Command`; skip + log when not alive.

## 2. Tests
- [x] 2.1 cd'd-mid-session → resumes from the filed dir, not the subdir.
- [x] 2.2 missing transcript → reported dead so restore skips it.
- [x] 2.3 non-claude agent unchanged; transcript without cwd keeps the caller's cwd.

## 3. Verify
- [x] 3.1 `make check` + `scripts/check-design.sh` green.
