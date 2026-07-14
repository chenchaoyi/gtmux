# Tasks — HQ chief-of-staff

## 1. Event severity (CODE — `internal/events`)

- [x] 1.1 Add `Severity string` (additive, `omitempty`) to `events.Record`
- [x] 1.2 `func Severity(r Record) string` — deterministic `routine|notable|important` classifier (event/state/kind/class), + `severityRank(level) int`
- [x] 1.3 Stamp severity in `Append` when the record carries none (source-stamped, all writers uniform)
- [x] 1.4 Unit tests: waiting→important, asking→important, report→notable, prompt→routine, lifecycle→notable; `Append` stamps + round-trips; rank ordering

## 2. `gtmux events --severity` filter (CODE — `internal/app/eventscmd.go`)

- [x] 2.1 Parse `--severity <level>` / `--severity=<level>` (invalid level → usage error)
- [x] 2.2 Filter the printer to records at that level and above (bare + `--follow` both honor it)
- [x] 2.3 Update `eventsUsage()` help (en+zh) to document `--severity`
- [x] 2.4 Test: `--severity important` yields only important records; invalid level rejected

## 3. Situation board scaffold (CODE — `internal/app/hq.go`)

- [x] 3.1 `hqNotesDir()` + `hqNotesSeeds` map with `board.md` template (per-ship: task · mode/source · priority · health · pending · lesson)
- [x] 3.2 `seedHQNotes()` (write-when-absent, never clobber) called from `seedHQHome`, contributing to the `seeded` return
- [x] 3.3 Test: `board.md` seeded, curated content never overwritten on re-seed

## 4. Corrections KB topic (CODE — `internal/app/hq.go`)

- [x] 4.1 Add `corrections.md` to `hqKnowledgeSeeds` (trigger→distill→land template) + list it in the KB `README.md` seed
- [x] 4.2 Extend `TestSeedHQKnowledge` to require `corrections.md`

## 5. Seed playbook — four new policy sections (PROMPT — `internal/app/hq.go` `hqInstructions`)

- [x] 5.1 §Posture: maintain `notes/board.md`; re-read after a context reset; query `events --severity important` + digest, not raw transcripts
- [x] 5.2 §Decision authority: the three command modes + the autonomy matrix (reversible∧low-risk∧in-discussed-scope → decide+dispatch; irreversible/permission/plan-fork/out-of-scope → escalate)
- [x] 5.3 §Graded escalation + reconcile: routine→board / important→coalesced summary / critical→push (existing pipeline); reconcile against live digest before relaying a needs-you
- [x] 5.4 §Learning loop: correction / repeated-footgun → distill to KB (portable) or notes (machine-specific), flag charter-level lessons
- [x] 5.5 Add `gtmux events --severity` to the Toolbox section
- [x] 5.6 Tests: `TestHQPlaybook*` assert the seed contains each new section's anchor phrases (en+zh)

## 6. Spec deltas

- [x] 6.1 `session-events`: ADDED "Events carry a deterministic severity" + severity-filtered read
- [x] 6.2 `supervisor-agent`: ADDED situation board · decision-authority tiers · graded escalation + reconcile · learning loop; MODIFIED KB scaffold (corrections.md)
- [x] 6.3 `openspec validate hq-chief-of-staff --strict` passes

## 7. Docs + gate

- [x] 7.1 `CLAUDE.md`: extend the HQ/中控 description with posture board · severity ledger · decision tiers · reconcile · learning loop
- [x] 7.2 `make check` green (gofmt + vet + staticcheck + `go test -race`)
- [x] 7.3 Branch → PR → CI green → squash-merge to main

## 8. Sync + archive

- [x] 8.1 `openspec sync-specs` (fold deltas into live specs)
- [x] 8.2 `openspec archive hq-chief-of-staff`; live specs validate `--strict`
