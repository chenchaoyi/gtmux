# Tasks

## Code (this PR)
- [x] tunnel-worker: per-device accounts at `/direct/redeem` (registry, port assignment, per-code device cap)
- [x] tunnel-worker: `GET /direct/authfile` behind `DIRECT_SYNC_TOKEN`, one older account per port
- [x] tunnel-worker: `revoke-direct-code.sh` removes the code and its accounts
- [x] tunnel-worker tests: isolation, idempotent re-redeem, device cap, port collision, authfile shape, revocation
- [x] client: store the assigned port; prefer it over the derived one
- [x] client: pre-check the account once before the long-running tunnel; explain a refusal
- [x] client: explain a redeem refused for too many devices
- [x] deploy/self-tunnel: chisel on the authfile only, a static service user, the 10s sync timer with atomic swap
- [x] deploy/self-tunnel: install-server.sh upgrades a pinned chisel version, provisions the sync token
- [x] end-to-end: chisel 1.12.1 server fed the Worker's authfile shape; own port only, no loopback, no public bind, revocation by restart, the sentinel (internal/app/tunnelaccount_test.go)
- [x] the sentinel account, because chisel with zero users skips authentication and every rule
- [x] revocation restarts chisel: a reload leaves established reverse tunnels serving
- [x] docs: deploy/self-tunnel/README.md, the operator cutover

## Cutover (operator; production)
- [ ] deploy the Worker and set `DIRECT_SYNC_TOKEN`
- [ ] run install-server.sh on the VPS with the same token
- [ ] re-redeem the operator's Mac and the Phase 0 device
- [ ] delete `DIRECT_SECRET`
- [ ] archive this change
