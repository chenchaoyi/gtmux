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

## Cutover (operator; production), 2026-09-22
- [x] deploy the Worker and set `DIRECT_SYNC_TOKEN`
- [x] run install-server.sh on the VPS with the same token (`CADDY=skip`: this box's :443 is an SNI router's)
- [x] re-redeem the operator's Mac
- [ ] re-redeem the other device that was on the shared secret (port 40953): its owner's action; disconnected until then
- [x] delete `DIRECT_SECRET`
- [x] archive this change
