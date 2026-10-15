# Codex session path scan runtime evidence

Date: 2026-10-15 (Asia/Shanghai)
Branch: `perf/codex-session-scan-index`
Baseline implementation: committed `HEAD` before product edits (`38cac61`); optimized implementation: current worktree.
Command: `agents --json`, using the same `$HOME/.codex` as `CODEX_HOME` for both versions.

## Matched final comparison

The active Codex session set was 20 for both five-sample runs. Every invocation exited 0, returned valid JSON, and reported 20 active Codex rows. The first baseline and optimized JSON outputs had identical row order and identical values for `agent`, `status`, `task`, `source`, `session_id`, `pane_id`, `client`, `can_read_conversation`, `errored`, `error_text`, `end_reason`, and `adoptable`.

| Version | Elapsed samples (s) | Median (s) | p95 (s, nearest rank) | CPU samples (s) | CPU median (s) | CPU p95 (s) |
|---|---|---:|---:|---|---:|---:|
| Baseline | 6.6675, 6.7864, 6.6571, 6.6456, 7.1067 | 6.6675 | 7.1067 | 6.7310, 6.8574, 6.7268, 6.6970, 7.1761 | 6.7310 | 7.1761 |
| Gather-scoped index | 0.0537, 0.0415, 0.0403, 0.0394, 0.0397 | 0.0403 | 0.0537 | 0.0501, 0.0452, 0.0440, 0.0431, 0.0434 | 0.0440 | 0.0501 |

Median elapsed time improved by approximately **165x** (99.4% lower); median CPU time improved by approximately **153x** (99.3% lower).

## Required pre-edit baseline

Before product-code edits, five baseline samples were captured with 23 active Codex sessions. All five exited 0 with valid JSON and consistent row order. Elapsed samples were 7.0606, 7.0896, 7.3316, 7.8326, and 8.3973 seconds (median 7.3316, p95 8.3973). CPU samples were 7.1273, 7.1736, 7.4264, 7.8719, and 8.4580 seconds (median 7.4264). Maximum counted Gtmux/check-out processes during that run: 5.

## Process guard

An independent monitor counted live processes whose executable/command identified this checkout or `/Applications/Gtmux.app/`. During the final matched run, the maximum observed count was **7**; the stop-at-9 and terminate-at-10 guard did not trigger. No validation workload was started at count 9 or above. The initial pre-edit run maximum was 5.

## Interpretation

The final run confirms one candidate-tree discovery per gather is sufficient for this workload. This optimization still enumerates inactive rollout filenames once per gather; it does not add cross-gather caching. Runtime samples are local-machine measurements and will vary with machine load and active session set.

## Clean local-install observation

After the test fix was committed, `make install` installed the clean commit to the local gtmux CLI path (`gtmux v1.0.115-5-g57edd25f`). Twenty consecutive `agents --json` invocations used the same Codex home. All exited 0, returned valid JSON, and reported 21 Codex rows.

| Metric | 20-run median | p95 (nearest rank) |
|---|---:|---:|
| Elapsed time | 0.0973 s | 0.1056 s |
| Child CPU time | 0.0882 s | 0.0935 s |

An independent monitor counted processes whose executable identified the CLI, checkout, or Gtmux app bundle every 100 ms. Per-scan post-run counts ranged from 3 to 5; the monitor peak was 6, the count ended at 3, and the stop-at-9 / terminate-at-10 guard did not trigger. The count did not increase across the repeated scans.
