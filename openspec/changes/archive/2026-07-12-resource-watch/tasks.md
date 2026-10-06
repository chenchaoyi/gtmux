# Tasks — resource-watch

## Reading these completion marks on 2026-10-06

The checkboxes below are preserved historical records. Checked item 1.1's Linux
fallback and item 2.2's digest-row fields are not present in the current source:
see the [sampler](../../../../internal/resource/sample.go),
[digest row](../../../../internal/radar/digest.go), and
[usage report](../../../../internal/radar/usage.go). Memory tier uses `sysctl`,
not `memory_pressure -Q`. The original 6.2 acceptance remains unchecked; archive
placement is not evidence that it passed. The
[current requirements](../../../specs/resource-watch/spec.md) remain in force.

- [x] 1.1 `internal/resource`: machine snapshot — df (disk free on the volume),
      `memory_pressure -Q` → normal/warn/critical tier, loadavg÷ncpu. Linux
      fallbacks (/proc, loadavg). Pure parsers, unit-tested on fixtures.
- [x] 1.2 Per-agent attribution: walk the pane-PID process tree (ps), sum RSS+CPU%.
      Reclaim candidates = heavy procs not under any live pane (curated reclaimable
      patterns — see design decisions). Tests on a synthetic ps snapshot.
- [x] 1.3 Layered thresholds (config): disk amber/red (% + GB floor), load ratio,
      memory tier. Pure Evaluate → per-resource warn string.
- [x] 2.1 `gtmux resource [--json]`; resource block on gatherUsage/digest → /api/usage.
- [x] 2.2 Per-agent RSS/CPU additive fields on digest rows.
- [x] 3.1 Serve-tick evaluator: sample + eval + resource·warn nudge (single-writer,
      atomic dedup marker). MOVE limits·warn's nudge here too (fixes the 3× race).
- [x] 3.2 HQ playbook + knowledge: weigh resources when dispatching; on severe,
      recommend reclaim (name orphans) or hold new sessions.
- [x] 4.1 Mobile HQ card/status strip: a resource line.
- [x] 5.1 Pre-flight: `gtmux hq`/`new` warn at a red-line resource.
- [x] 6.1 Docs (cli.md/CLAUDE.md); sync-specs + archive.
- [ ] 6.2 make check green; dogfood: disk ~40 GiB → amber; a real orphan (leftover
      simulator/dev-server) named; one nudge per crossing (no 3×).
