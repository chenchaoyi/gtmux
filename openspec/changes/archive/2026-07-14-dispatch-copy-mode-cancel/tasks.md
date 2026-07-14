# Tasks — Exit copy-mode before delivery; expose in_mode

- [x] 1. `internal/tmux`: add `InMode(pane)` + `ExitCopyMode(pane)` (gated on InMode)
- [x] 2. `internal/hqnudge`: converge `prod.inMode` onto `tmux.InMode` (no behavior change)
- [x] 3. `internal/dispatch`: add optional `InMode`/`ExitMode` to `IO`; paste guard exits copy-mode before each paste
- [x] 4. `internal/app/dispatchbridge.go`: bind the new IO hooks to `tmux.InMode`/`tmux.ExitCopyMode`
- [x] 5. `gtmux send` plain + `--key` paths: `ExitCopyMode` before the write
- [x] 6. `POST /api/send` (`sendToPane`): `ExitCopyMode` before the write
- [x] 7. `agents --json` (`agentJSON`) + gather: add `in_mode` from `#{pane_in_mode}`
- [x] 8. `digest --json` (`digestRow`) + `gatherDigest`: carry `in_mode`
- [x] 9. Tests: dispatch exits copy-mode before paste (and does NOT cancel a non-mode pane); `in_mode` present/omitted on both contracts
- [x] 10. Spec deltas: agent-dispatch (delivery exits copy-mode), agent-radar + agent-digest (`in_mode`)
- [x] 11. `make check` (gofmt + vet + staticcheck + `go test -race`) green; `openspec validate --specs --strict` green
- [ ] 12. Branch → PR → CI green → squash-merge → sync-specs + archive-change
