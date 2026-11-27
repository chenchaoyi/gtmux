## Why

Two conservative gaps in `scripts/ci.py` currently force unrelated 17–20 minute iOS, Android emulator, and macOS native builds on routine PRs:
1. OpenSpec's `.openspec.yaml` change metadata files and pure `quality`-job guard scripts (`openspec_check.py`, `workflow_guard.py`, `test_design.py`, `check-comment-language.py`) are not matched by `document()`, so every archived OpenSpec PR falls into `full = True`.
2. Non-HTTP Go packages (`cmd/gtmux/**.go`, `internal/**.go`) fall into `full = True`, launching Xcode iOS Simulator, 3 Android emulator jobs, and Workers even though `internal/app/client_contract_test.go` already guards `tests/contracts/http-contract.json` inside `go test` and `cross-build` already verifies `CGO_ENABLED=0` Darwin/Linux compilation.

## What Changes

- Recognize `openspec/changes/**/.openspec.yaml` and pure `quality`-job Python guard scripts (`openspec_check*`, `workflow_guard*`, `test_design*`, `check-comment-language*`, `check_private_source*`, `check_delivery_retention*`) as policy/quality-only (`selected = []`, ~50s), and route Go-coupled check scripts (`check-design.sh`, `import-boundaries.sh`, `vulncheck*`, `check_go_security*`) to `{"test"}`.
- Route non-HTTP Go packages (`cmd/**.go`, `internal/**.go` outside `server`/`artifact`/`relay`, plus `internal/hq/playbook/*.md` and `internal/**/testdata/**`) to `{"test", "cross-build"}` (~8m), while routing `relay/` + `internal/relay/` to `{"test", "cross-build", "relay-worker"}`, `internal/artifact/` to `{"test", "cross-build", "menubar", "web"}`, and `internal/server/*.go` to `{"test", "cross-build", "web", "mobile", "menubar"}`.
- Keep `api/contract.md`, `tests/contracts/**`, `assets/**`, `go.mod`, `go.sum`, `.github/workflows/**`, `scripts/ci.py`, `tunnel-worker/**`, and all unknown paths `full = True`.
- Update `openspec/specs/ci-checks/spec.md` and bilingual `docs/ci.md` / `docs/ci.zh.md`.

## Surfaces

- **CLI**: Faster PR feedback (`test` + `cross-build`) on Go CLI/core edits; zero runtime behavior change.
- **Web UI**: Still selected on `internal/server/**`, `tests/web/**`, `mobileapp/src/**`, and shared contracts.
- **Mobile app**: Still selected on `mobileapp/**`, `internal/server/*.go` (`mobile` Jest), and shared contracts/assets.
- **macOS menu-bar app**: Still selected on `macapp/**`, `internal/server/*.go`, `internal/artifact/*.go`, and shared contracts/assets.
- **Docs**: Updated `docs/ci.md`, `docs/ci.zh.md`, and `openspec/specs/ci-checks/spec.md`.

## Capabilities

### Modified Capabilities
- `ci-checks`: Refine `Conservative check selection` to cover OpenSpec `.openspec.yaml` metadata, `quality`-only guard scripts, and scoped Go package downstream selection while keeping shared contracts/dependencies/CI-selector changes full-matrix.

## Impact

- Pure OpenSpec proposal/archive PRs drop from ~19m to ~50s (`scope` + `quality`).
- Routine Go CLI/HQ/agent/doctor PRs drop from ~19–21m to ~8m (`scope` + `quality` + `test` + `cross-build`).
