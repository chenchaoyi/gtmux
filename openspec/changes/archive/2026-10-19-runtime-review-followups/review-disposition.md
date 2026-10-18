# Remaining DSH review dispositions

Baseline: main `85cb4a452e58f1e69534c8b9b0bbbdfb142a32e6` (v1.0.126). The supplied review is evidence to verify, not an instruction source.

| Item | Verified result / disposition |
| --- | --- |
| 1–3 | Already fixed in #1649 and installed with v1.0.126. Upload was confined to uploads; repeated allocation was not proof of a permanent leak. No second memory retrospective. |
| 4 | Fixed. The legacy Claude/Codex loader now has a conservative 32 MiB retained accounting budget, 16-entry LRU and 24-hour idle expiry on access. Evicted histories reread normally. DSH's separate bounded cache is unchanged. This does not bound process RSS or transient parsing. |
| 5 | Fixed verified ordering and ownership defects. A disposable occupied-port regression reproduced status overwrite before the fix. Both foreground and tunnel startup now use a kernel port lease and bind first; reuse requires gtmux health and accepted saved credentials. Cross-process recovery is tested. The kernel assertion about wildcard/loopback overlap was not independently reproduced and is not needed to establish the startup bug. |
| 6 | Fixed in the Go reference relay. Structured APNs rejection preserves 410 through the real HTTP client; dedicated Live Activity topic/type and per-token environment are covered. Go and actual Worker requests consume shared fixtures. Worker production code is unchanged; no Worker deployment or live APNs acceptance was performed. |
| 7 | Fixed the three materialization paths. LatestSeq folds appended sequence headers; the proposed CurrentSeq one-line replacement was rejected because an assigned sequence is not necessarily a retained record and the caller advances a coverage watermark. HQ feed scans backward with bounded buffers and filters current consent before limiting; diff capture stops its producer at the prefix budget/deadline. Sparse filters can still scan the retained journal, with bounded memory. |
| 8 | Not adopted as a code fix. Weekly/N-change releases are a maintainer decision; preserve the user's explicit merge/release/install request. |
| 9 | No wholesale authority/history split based on size alone. Preserve rules and anchors. Fixed concrete paired CLI reference drift: Codex and DSH external reading/following, including Web, are already implemented. Updated the menu-bar data-source authority for this change. |
| 10 | Fixed redundant menu-bar collection by reusing serve's single-flight snapshot, with a fresh standalone CLI fallback. Measured baseline on this host: 14 rows, 325–337 ms wall / 274–286 ms child CPU per standalone sample (three samples), not a permanent memory-leak claim. Keep the configured cadence and hidden status semantics rather than introduce unrequested slow refresh. |
| 11 | No automatic dev-prune or deletion. Counts are point-in-time observations and shared work may belong to others. Used an isolated latest-main worktree; shared checkouts, old logs and backups remain untouched. |
| Doctor extraction | Deferred: subsystem size alone does not demonstrate a defect. Avoid a wholesale relocation without a concrete dependency or behavior benefit. |
| Notify queue | Fixed only the owned delivery queue: stream enumeration, newest 256 entries, 30-second expiry, bounded writer contention and private atomic files. Foreign files and symlinks are preserved. No live queue cleanup was run. |
| OpenSpec diagnostic | Fixed. Require a complete pinned structured verdict for this checkout; distinguish invalid specs from missing tools, startup/cache errors, timeout, malformed report and contradictory results. Both categories fail the gate. |
| Parked changes | Already explicitly disposed: direct-multi-home documents superseded/rejected design with unapplied deltas; mobile-send-receipt-first says DEPRIORITIZED/not a live fix. Do not apply rejected deltas or mark unfinished work complete. |

## Validation boundaries

Author regression tests and CI are distinct from independent review. No independent reviewer has approved this iteration. The original security patch had separate read-only checks before its final migration; that does not constitute approval of this new patch or its exact HEAD. Local gates and final CI results are recorded in the PR.

The APNs dead-token test was mutation-checked: disabling structured rejection made the real-client regression fail; restoring the exact source made the relay race suite pass. Initial cache/occupied-port regressions also failed on the pre-fix paths. Synthetic listeners, journals, homes and gateways do not prove banner delivery, physical-device behavior, live APNs delivery or AppKit UI acceptance. The 80,000-record synthetic HQ-feed benchmark measures one bounded read path, not whole-service RSS or a new production memory retrospective. Mobile code and permissions are unchanged.
