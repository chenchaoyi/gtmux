## Implementation

- [x] Add the Standard service repair flag and Worker repair receipt.
- [x] Reapply ingress and reconcile the existing managed DNS route safely.
- [x] Cover missing DNS, stale routes, provider errors, and CLI compatibility.
- [x] Update the remote-access spec and bilingual CLI docs.
- [x] Run the repository gate and Worker checks; record environment failures separately.

## One-action recovery

- [x] Add confirmed-missing-only recovery to the Worker and CLI.
- [x] Add the single Restore connection action to the pairing window.
- [x] Verify changed addresses, failed commands and honest reachability in model tests.
- [x] Update the design, specs and bilingual docs for the simplified flow.

## Delivery

- [x] Deploy the updated tunnel Worker with existing variables and secrets preserved.
- [x] Prepare the CLI and menu-bar recovery for release v1.0.73.

## Validation

- Worker: 44 tests and TypeScript checks passed.
- Mac: 288 tests passed; release build checked.
- CLI: focused repair and recovery tests, cgo-free build, formatting, vet and staticcheck passed.
- Full local `make check`: existing macOS/environment-sensitive tests fail outside
  the repair path; CI must pass before merge.
- Cloudflare deployment: authenticated and deployed on 2026-10-03; version
  `3749eecb-02ae-4de2-bd58-1ee7b7f2fad5` passes control-plane health and auth checks.
- Full PR CI: all six jobs passed, including Go race tests and macOS release/model checks.
- The Mac Standard address and localhost return the same healthy gtmux boot.
