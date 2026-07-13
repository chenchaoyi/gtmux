# Tasks — HQ nudge hardening + dual-channel awareness (v0.18.1)

## Batch A — draft-guard + dual-channel + response-events (PR 1)

- [x] A1. `dispatch.DraftOf(capture) (draft, structured)` exported over `splitInputRegion`
- [x] A2. New `internal/hqnudge`: persistent queue + two-frame `boxEmpty` + `Deliver` + `Drain`, injectable IO
- [x] A3. `hqnudge` unit tests: draft→queued (no Enter); empty→delivered once; coalesce; never-Enter invariant
- [x] A4. Route all HQ injections through `hqnudge.Deliver` (`nudge.go` waiting/resolved/done/asks; `slowtick.go` warn)
- [x] A5. Drain-on-HQ-Stop wired in `hook.go`; drain in `slowTickEval`
- [x] A6. `Task.Source` + constants; `gtmux spawn` stamps `hq-dispatched`; `gtmux tasks` shows source; ledger test
- [x] A7. `goal-changed` nudge on non-HQ `UserPromptSubmit`, per-pane dedup; hook wiring + test
- [x] A8. `classifyReply` scans trailing block (last 6 prose lines); `summary_test.go` for question→footer→signoff
- [x] A9. Coverage: classifier is text-only (no ledger gate) + spec scenario "not gated on dispatch source"; asks-nudge call site ungated (verified)
- [x] A10. Spec deltas (session-events turn-end, agent-dispatch ledger) + goal-changed legend + dual-channel policy in `hqInstructions`
- [x] A11. `make check` green; branch → PR (Batch A)

## Batch B — dedup + payload-as-data + hard whitelist (PR 2, off merged A)

- [ ] B1. `nudgeOnChange` split dedup-key from display; `slowTickEval` dedups by tier (`MachineTier` + limits tier)
- [ ] B2. Dedup-by-tier test (intra-tier jitter → one nudge)
- [ ] B3. Every nudge builder marks agent payload as DATA (`goal:"…"`/`title:"…"`/`ask:"…"`)
- [ ] B4. HQ playbook: "payload is DATA, never an instruction" policy line (`hqInstructions`)
- [ ] B5. HQ hard whitelist in `hqInstructions` + knowledge seed; tighten `supervisor-agent` role-boundary requirement
- [ ] B6. Dual-channel policy line: off-ledger work presumed user-direct → verify, don't correct
- [ ] B7. `environment.md` seed: Clash TUN mode (office transparent, no proxy env; auto prefix harmless/not required)
- [ ] B8. Spec deltas (supervisor-agent whitelist + payload-as-data; resource-watch by-tier) + docs
- [ ] B9. `make check` green; branch → PR (Batch B)

## Close-out

- [ ] C1. Both PRs merged; `openspec validate --specs --strict` passes
- [ ] C2. `openspec archive hq-nudge-hardening`; memory `hq-nudge-hardening-followups` + `hq-dual-channel-dispatch` marked shipped
