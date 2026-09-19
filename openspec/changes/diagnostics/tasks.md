# diagnostics — tasks

## Phase 1 — the store, and the incident that exposed its absence

- [x] 1.1 `internal/diag`: the store (one file per local day under `logs/`), the entry
      schema of design 4.1 with `kind` `diag` and `act`, atomic single-write appends of at
      most 4 KB, a logging failure never failing its caller; tests
- [x] 1.2 Redaction where the line is written (design 4.2): registered secrets,
      credential-named attrs, `#c=` / `#g=` fragments, `Authorization` values; a test that
      logs every secret in every position and finds none; no user content by construction
      (sends record length and a short hash)
- [x] 1.3 Retention and cleanup (design 5.1, 5.2): 30 days / 100 MB, settable; run by the
      first writer of a new day under a marker and by serve's slow tick; the 20 MB in-day
      guard with a `log.runaway` entry; the 50 MB debug drop; each deletion an
      `act.cleanup` entry; tests with a fixture store
- [x] 1.4 Status files (design 7.1): atomic write, `updated` + `staleAfter`, a reader that
      reports stale as unknown; tests
- [x] 1.5 Private by default (design 3.2): `umask 077` at the Go entry point and at the menu
      bar app's launch; a helper for files gtmux edits that belong to something else,
      used by every such writer; a test that the entry point sets the umask
- [x] 1.6 Narrowing: both roots to 0600 / 0700 on serve start and on every disk-hygiene
      sweep, skipping what cannot be changed; each change logged as `act.narrow`; a test
      against a fixture tree
- [x] 1.6a Layout (design 3.1): `state.ConfigDir()` and every gtmux path built through
      `state`; `Paths.swift` in the menu bar app; `tunnel-url` moved to the data root with a
      one-release fallback; `icon-cache/` and `agent-icons/` into `cache/`; `hq-feed/` and
      the empty `briefs/` removed, logged as `act.cleanup`
- [x] 1.7 serve: diagnostics (start, stop, `auth.rejected` aggregated per minute,
      `enroll.*`, `handler.panic`, `push.failed`) and actions with the device as actor
      (pairing, revoke, send, focus, upload, share, push register and forget);
      `status/serve.json`; a test per event
- [x] 1.8 Direct tunnel client: `status/tunnel.json` from the chisel connection and a
      30-second end-to-end probe of its own pairing URL; `tunnel.*` entries
- [x] 1.9 Standard tunnel: cloudflared started with an explicit `--metrics`; serve's slow
      tick reads `cloudflared_tunnel_ha_connections` into `status/tunnel.json`
- [x] 1.10 Menu bar: the pairing window explains an unreachable address from
      `status/tunnel.json`; the `tunnel.log` phrase match removed; the Preferences door
      status shows the tunnel state
- [x] 1.11 `gtmux logs` (design 9): last hour by default, `--since`, `--until`,
      `--component`, `--level`, `--acts`, `--actor`, `--event`, `--follow` across midnight,
      `--json`; the command table, CLAUDE.md command list, `docs/cli.md` both halves
- [x] 1.12 `gtmux doctor`: the `Logs` section of design 8 with its `--fix` actions
- [x] 1.12b `gtmux doctor`: the tunnel row; the cloudflared row only under Standard; "the
      phone can reach this Mac" only when status shows it
- [x] 1.13 `check-design.sh`: no gtmux path built outside `internal/state` (or `Paths.swift`)
- [x] 1.13b `check-design.sh`: no surface reading a `*.log` for state
- [x] 1.13a `gtmux doctor --fix` offers to remove the credential backups (`*.bak-*`) left in
      the config root by earlier migrations
- [x] 1.14 Specs synced; `docs/TROUBLESHOOTING.md` points at `gtmux logs` and status
      instead of `tail tunnel.log`

## Phase 2 — the full trail

- [x] 2.1 The action catalog: an event for every command the command table marks as
      writing, and a test that fails a writing command with none
- [x] 2.2 Actions for every writing command, with actor attribution (user, `agent:%N`, HQ)
- [x] 2.3 HQ and housekeeping acts; the journal's seven audit acts written through the same
      call as their log entry
- [x] 2.4 Menu bar: entries in the shared schema written to the store, mirrored to the
      unified log under subsystem `com.gtmux.menubar`
- [x] 2.5 hook and restore on the store; `hook.log` and `restore.log` retired
- [x] 2.6 `GTMUX_DEBUG=<components>|all` and config `debug`; the old variables as aliases
- [x] 2.7 LaunchAgent captures under `logs/<component>.stderr` when plists are regenerated;
      disk hygiene follows them
- [x] 2.8 `mine/` bounded: signatures unseen for 90 days pruned, `passes.jsonl` rotated
- [x] 2.9 Tunnel down and back up as a journal control record, taught in the HQ playbook
      (bump `hqPlaybookVersion`) and in `docs/cli.md`'s class table

## Phase 3 — leaving the machine

- [ ] 3.1 Phone and iPad: a buffer of the last 500 entries (200 KB on disk) in the shared
      schema, redacted, under Settings → Diagnostics with copy and share; jest tests for
      redaction and the cap
- [ ] 3.2 `gtmux doctor --bundle <path>`: `logs/`, `status/`, the doctor report and versions;
      the journal only with `--with-events`; prints what it packed
- [ ] 3.3 Archive the change
