## Tasks
- [x] Add expiring, exact-payload delivery intents with live-target guards.
- [x] Integrate spawn, hook attribution and radar reconciliation.
- [x] Filter binding metadata from parsed prompts.
- [x] Add regression tests for ownership and failure guards.
- [x] Align capability specs and English/Chinese Codex docs.
- [x] Run repository/design checks, review and archive the change.

## Verification (2026-10-03)
- Targeted Go tests: resume, transcript, hook, radar and app passed.
- `make check`: gofmt, vet, staticcheck and the full race suite passed.
- `bash scripts/check-design.sh`: OpenSpec, architecture, cgo-free build and documentation checks passed.
- Scope review: only interactive unbound Codex spawn adds an intent; submitted user payloads are distinct from assistant/tool echoes; rejected witnesses cannot fall back to cwd attribution; target and incumbent guards precede writes.
- Existing-worker repair: independent phone delivery fingerprints/counts/times uniquely identified the rollout, unchanged live pane identity was verified, and its missing resume record was created exclusively. Native duplicate disappeared; authenticated local Chat API returned seven prompts and seven responses. No worker reset, keystroke, build installation or release.
- Five surfaces share Go radar/transcript. No mobile/native UI build or physical-device layout acceptance was claimed.
