# Tasks — resource-watch

- [ ] 1.1 `internal/resource`: machine snapshot — df (disk free on the volume),
      `memory_pressure -Q` → normal/warn/critical tier, loadavg÷ncpu. Linux
      fallbacks (/proc, loadavg). Pure parsers, unit-tested on fixtures.
- [ ] 1.2 Per-agent attribution: walk the pane-PID process tree (ps), sum RSS+CPU%.
      Reclaim candidates = heavy procs not under any live pane (curated reclaimable
      patterns — see design decisions). Tests on a synthetic ps snapshot.
- [ ] 1.3 Layered thresholds (config): disk amber/red (% + GB floor), load ratio,
      memory tier. Pure Evaluate → per-resource warn string.
- [ ] 2.1 `gtmux resource [--json]`; resource block on gatherUsage/digest → /api/usage.
- [ ] 2.2 Per-agent RSS/CPU additive fields on digest rows.
- [ ] 3.1 Serve-tick evaluator: sample + eval + resource·warn nudge (single-writer,
      atomic dedup marker). MOVE limits·warn's nudge here too (fixes the 3× race).
- [ ] 3.2 HQ playbook + knowledge: weigh resources when dispatching; on severe,
      recommend reclaim (name orphans) or hold new sessions.
- [ ] 4.1 Mobile HQ card/status strip: a resource line.
- [ ] 5.1 Pre-flight: `gtmux hq`/`new` warn at a red-line resource.
- [ ] 6.1 Docs (cli.md/CLAUDE.md); sync-specs + archive.
- [ ] 6.2 make check green; dogfood: disk ~40 GiB → amber; a real orphan (leftover
      simulator/dev-server) named; one nudge per crossing (no 3×).
