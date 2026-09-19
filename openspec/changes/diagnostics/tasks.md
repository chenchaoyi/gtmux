# diagnostics — tasks

## Phase 1 — foundations, and the Direct misdiagnosis

- [ ] 1.1 `internal/diag`: the logger (schema of design 4.1, levels, per-component files,
      rotation by rename at 2 MB keeping two generations) with tests
- [ ] 1.2 Redaction at the sink (design 4.2): registered secrets, credential-named attrs,
      `#c=` / `#g=` fragments, `Authorization:` values; a test that logs every secret in
      every position and finds none
- [ ] 1.3 Status files (design 5.1): atomic write, `updated` + `staleAfter`, a reader that
      reports stale as unknown; tests for both
- [ ] 1.4 `state` helpers that create files 0600 and directories 0700; move gtmux-owned
      writers onto them
- [ ] 1.5 One-time migration on first start: narrow `~/.local/share/gtmux/` and
      `~/.config/gtmux/` (dirs 0700, files 0600, leaving files gtmux does not own alone),
      remove `hq-feed/spool.jsonl`, write a marker; test against a fixture tree
- [ ] 1.6 serve: the events in design section 6 at their default levels, `auth.rejected`
      aggregated to at most once a minute; `status/serve.json`; a test per event
- [ ] 1.7 Direct tunnel client: `status/tunnel.json` from the chisel connection plus a
      30-second end-to-end probe of its own pairing URL; `tunnel.*` log events
- [ ] 1.8 Standard tunnel: start cloudflared with an explicit `--metrics 127.0.0.1:<port>`;
      serve's slow tick reads `cloudflared_tunnel_ha_connections` and writes
      `status/tunnel.json`
- [ ] 1.9 Menu bar: the pairing window's "why can't it reach" reads `status/tunnel.json`
      (connected and fresh, down with the recorded error, stale or missing); remove the
      `tunnel.log` phrase match; the Preferences door status shows the tunnel state
- [ ] 1.10 `gtmux doctor`: a tunnel row (backend, state, since, last error); the cloudflared
      row only under Standard; "the phone can reach this Mac" only when status shows it
- [ ] 1.11 `check-design.sh`: fail a literal world-readable mode in `internal/` outside an
      allowlist; fail a surface that reads a `*.log` for state
- [ ] 1.12 Specs synced (`diagnostics`, `menu-bar-app`, `env-doctor`); `docs/TROUBLESHOOTING.md`
      points at status and `logs/` instead of `tail tunnel.log`

## Phase 2 — reading

- [ ] 2.1 `gtmux logs [component…] [--level] [--since] [--follow] [--json]`, merged by time;
      the command table, CLAUDE.md command list, `docs/cli.md` section in both halves
- [ ] 2.2 `GTMUX_DEBUG=<components>|all` and config `debug`; `GTMUX_HOOK_DEBUG`,
      `GTMUX_TUNNEL_DEBUG`, `GTMUXBAR_DEBUG` kept as aliases
- [ ] 2.3 hook: `hook.error` at warn by default, trace behind the switch; retire `hook.log`
- [ ] 2.4 restore: move `restore.log` onto the logger
- [ ] 2.5 LaunchAgent captures move to `logs/<component>.stderr` when plists are
      regenerated; `diskhygiene` follows them
- [ ] 2.6 Menu bar: a small logger writing `logs/menubar.jsonl` in the shared schema, mirrored
      to the unified logging system (subsystem `com.gtmux.menubar`)
- [ ] 2.7 Tunnel down and back up written to the journal as a control record, taught in the
      HQ playbook (bump `hqPlaybookVersion`) and in `docs/cli.md`'s class table
- [ ] 2.8 Bound `mine/`: prune error signatures not seen for 90 days; rotate `passes.jsonl`

## Phase 3 — leaving the machine

- [ ] 3.1 Phone and iPad: a ring buffer of the last 500 entries (200 KB on disk), redacted,
      under Settings → Diagnostics with copy and share; jest tests for redaction and the cap
- [ ] 3.2 `gtmux doctor --bundle <path>`: `logs/`, `status/`, the doctor report and versions;
      the journal only with `--with-events`; prints what it included
- [ ] 3.3 Archive the change
