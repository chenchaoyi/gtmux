## Context

`scripts/ci.py` implements conservative module selection in front of `ci-required`. Auditing actual job consumers in `.github/workflows/ci.yml` established exact boundaries:
1. `quality` always runs `make ci-test publisher-test` and `./scripts/check-design.sh --source-only`, so OpenSpec `.openspec.yaml` files and `quality`-only guard scripts are already 100% exercised without spinning up platform runners.
2. `internal/app/client_contract_test.go` (run in `test`) compares real Go HTTP handler outputs against `tests/contracts/http-contract.json`, while `mobile` and `web` independently test their fakes against the same contract file. Because `tests/contracts/**` and `api/contract.md` stay `full = True`, an undeclared Go wire change fails in `test`, and an intentional contract update runs the full matrix.
3. `macapp/Tests/GtmuxBarTests/LocalArtifactPreviewTests.swift` executes the bundled CLI's `serve` and `artifact add/open`, so `internal/server/*.go` and `internal/artifact/*.go` continue to select `menubar`.
4. `relay-worker` shares its APNs payload shape with `relay/` (`internal/relay/`), so those Go files select `relay-worker`.

## Decisions

1. **Allowlist only exact OpenSpec change `.openspec.yaml` paths**: `openspec/(?:changes/(?:archive/)?[a-z0-9][a-z0-9-]*/)?\.openspec\.yaml` (never arbitrary `.yaml` files).
2. **Keep `scripts/ci.py` and `scripts/ci_test.py` as `full = True`**: Any edit to the scope selector itself (including this PR!) runs all 16 checks to prove the workflow wiring end-to-end.
3. **Preserve downstream release/delivery invariants**: Desktop `release.yml` requires `validate_ci` (`main` push workflow `conclusion == "success"`), which holds when unselected jobs skip cleanly behind `ci-required`; Android test delivery (`validate_android_jobs`) runs after `set-version.sh` stamps `mobileapp/src/version.ts`, which continues to select all Android and iOS jobs.
