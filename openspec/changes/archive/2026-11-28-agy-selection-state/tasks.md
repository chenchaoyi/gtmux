## 1. Implementation

- [x] 1.1 Update `internal/prompt/prompt.go` and `internal/prompt/prompt_test.go` for navigation-confirmed `>` menus, narrow-pane soft-wrap continuation joining, 240-rune replyable option labels, and `agy` project trust startup gate.
- [x] 1.2 Extend `internal/radar/agents.go`, `internal/radar/digest.go`, `internal/hq/slowtick.go`, `internal/dispatch/deliver.go`, and `internal/app/serve.go` (`hasPendingAsk`) to promote and clear `agy` screen approval waits immediately.
- [x] 1.3 Add unit/component tests in `internal/radar/agy_wait_test.go` and `internal/app/pendingask_test.go`, refresh the test catalogue, archive OpenSpec, and pass `make check` and `--pr-history`.
