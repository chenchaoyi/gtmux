# Tasks — hq-self-rotate

## 1. Sensing primitives

- [x] 1.1 `internal/transcript`: add `FirstMessageTime(agent, sessionID)` — the head-read
      twin of `LastMessageTime`, so session age comes from the transcript rather than from
      when gtmux happened to start watching.
- [x] 1.2 Test: first vs last timestamp on the same fixture log; missing/unparsable → 0.

## 2. Wake vocabulary + config

- [x] 2.1 `internal/hqwake/wake.go`: `ClassSelfRotate = "self-rotate"`, priority
      `PriorityStanding` (it re-fires on its own cadence, so evicting it costs nothing and
      it must never arrive ahead of a blocked agent).
- [x] 2.2 `internal/hqwake/config.go`: `SelfRotateCtx` (0.75), `SelfRotateHours` (12),
      `SelfRotateTurns` (300), `SelfRotateRepeatSec` (1800); a non-positive threshold
      disables THAT criterion only; a non-positive repeat falls back to the default.
- [x] 2.3 Test: defaults, per-field override, per-criterion disable, malformed file.

## 3. The sensor

- [x] 3.1 `internal/hq/selfrotate.go`: state file (session id / started-at / turns / event
      cursor / knocked-at), pure `selfRotateDecide`, and the slow-tick body.
- [x] 3.2 Session id change → window restarts (age, turns, cadence).
- [x] 3.3 Turn counting: HQ-pane `UserPromptSubmit` records past the stored cursor,
      accumulated incrementally so no tick re-scans the whole journal.
- [x] 3.4 Wire into `SlowTickEval`, after the completeness net.
- [x] 3.5 Tests: breach knocks · below thresholds does not knock · no HQ pane is a total
      no-op (no knock, no state written) · knocking does not clear the debt · a new session
      id does · a disabled criterion does not fire.

## 4. Rotation mechanism

- [x] 4.1 `gtmux hq --rotate`: resolve the HQ pane, exit copy-mode, deliver the agent's
      reset input (paste + Enter as separate steps), restart the health window.
- [x] 4.2 Non-zero exit with a clear message when no HQ pane is resolvable.
- [x] 4.3 `--help` text (en + zh).

## 5. Observability

- [x] 5.1 `hq.SessionHealthStatus(now)` — the read side (pure disk reads, safe with no tmux).
- [x] 5.2 `gtmux doctor`: `HQ session health` row in the HQ group, beside `event
      consumption` / `knowledge distill` / `HQ self-check`.
- [x] 5.3 Test: OK / flagged / no-HQ-is-informational.

## 6. Playbook + docs

- [x] 6.1 `hqInstructions`: the `self-rotate` class in the wake-class list, and the
      three-step ritual (board+KB current → hand off → `gtmux hq --rotate`), including the
      `UserPromptSubmit` vs `Stop` arbiter for the self-as-input failure.
- [x] 6.2 Bump `hqPlaybookVersion` 15 → 16 with its history entry.
- [x] 6.3 `docs/cli.md`: class-table row + a section on the mechanism + `gtmux hq --rotate`.
- [x] 6.4 `CLAUDE.md`: the sensor in the HQ paragraph.

## 7. Gate

- [x] 7.1 `make check` green (gofmt / vet / staticcheck / `go test -race`).
- [x] 7.2 `scripts/check-design.sh` green (the new class is taught in BOTH docs/cli.md and
      the seeded playbook).
- [x] 7.3 `openspec validate --specs --strict` green.
