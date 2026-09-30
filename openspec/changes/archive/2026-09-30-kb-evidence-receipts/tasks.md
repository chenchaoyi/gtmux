- [x] Preserve candidate identity, digest, metadata and available transcript references.
- [x] Commit acceptance/dismissal before settling pending work; serialize concurrent writers.
- [x] Add correlated outcomes, detail sources and committed receipt reads.
- [x] Cover legacy input, failed writes, retries, new same-key observations, concurrent
      processes, supersede history and source isolation with regression tests.
- [x] Update English/Chinese docs, API and capability spec; sync and archive this change.
- [x] Run targeted tests, mutation checks, make check, cgo-free build and design/spec gates.
- [x] Complete local review and prepare the PR; CI and merge status are tracked on the PR.

## Verification

- Targeted knowledge/mine/events/HQ suites passed.
- Removing source snapshots made the source-preservation test fail; disabling settled-ID
  filtering made the legacy/retry test fail. Both mutations were restored.
- `make check` passed (format, vet, pinned staticcheck, all Go tests with race detection).
- `scripts/check-design.sh` and pinned OpenSpec 1.10.0 strict validation passed (33 specs).
- `CGO_ENABLED=0 go build ./cmd/gtmux` passed. No mobile/native build required by this batch.
- Review caught and fixed a v3 ledger being mistaken for v1 by the backup helper;
  source-bearing files now use private permissions. Source lineage also survives a
  captured supersede, including recurrence lookup and counts.
