# Tasks: hq-attention-stream

## 1. The tier (code)

- [x] 1.1 `internal/events`: additive `Origin` field on the record + the
      `OriginInstruction` constant, documented as author-agnostic (a dispatched task
      carries an instruction too).
- [x] 1.2 `events.Severity`: a `UserPromptSubmit` carrying `OriginInstruction` → `notable`;
      without it → `routine`. Every other rule unchanged. Table test: instruction
      submission notable, injected submission routine, legacy record (no origin) routine,
      Waiting/asking/crash still important, report/lifecycle still notable.
- [x] 1.3 `internal/hook`: stamp `Origin` from `goalOf(payload.Prompt)` — the SAME call
      that decides the `goal-changed` wake, so the two can't disagree. Test: prose and
      slash both stamp; harness content and an echoed `» gtmux·` line don't.

## 2. The wording (prompt + docs)

- [x] 2.1 `hq.go` playbook v4: name the three reads (unfiltered `--since-seq` = the
      reconcile delta, `--severity notable` = fleet-change, `--severity important` =
      escalation subset) and add the rule "a filtered read is a triage shortcut, not your
      model of the world". Bump `hqPlaybookVersion` 3 → 4 with a history line.
- [x] 2.2 `hq_test.go`: the playbook fixture asserts the three reads and the shortcut
      rule; fix the stale comment on the `--severity important` assertion (the string
      survives, its meaning doesn't).
- [x] 2.3 `eventscmd.go`: `--severity` help + the filter comment stop calling it "the
      attention stream"; name the escalation subset and point at `--since-seq`.
- [x] 2.4 `events.go`: the `Severity` field comment stops claiming the filtered read is
      "the attention stream".
- [x] 2.5 CLAUDE.md + `docs/cli.md`: same correction wherever the claim is repeated.

## 3. Spec ⇄ code ⇄ test ⇄ docs consistency

- [x] 3.1 `make check` green; `scripts/check-design.sh` green;
      `npx @fission-ai/openspec validate --strict` green.
- [x] 3.2 Sync the deltas into `openspec/specs/` and archive this change once merged.
