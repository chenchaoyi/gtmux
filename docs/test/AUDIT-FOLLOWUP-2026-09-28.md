# Repository and knowledge audit follow-up (2026-09-28)

This follows the [initial audit](AUDIT-2026-09-28.md). Knowledge counts are a snapshot of the local HQ ledger and may change. Private entries were not committed to the repository.

| Area | Change | Verification |
| --- | --- | --- |
| HQ maintenance, restore logging, mobile accessibility, Web interactions | PR #1209 updated `internal/hq`, `internal/app`, `mobileapp`, `internal/server/web`, OpenSpec, and documentation. | `make check`, mobile JS checks, design check, and CI passed before merge. |
| Correlation and write failures | PR #1210 added `op_id` across `internal/events`, `internal/diag`, and `internal/knowledge`, surfaced append failures, and added doctor write probes. | Full `make check` and six CI jobs passed. A live knowledge supersede had the same `op_id` in the ledger, `gtmux:audit:knowledge` event, and `act.knowledge` diagnostic record. |
| Knowledge lint false positives and Jest exit warning | PR #1211 excluded example placeholders and explanatory `token=` text, cancelled mobile timers and animation frames on unmount, and retained the audit record after a render failure. | `make check`, design check, strict OpenSpec validation, 101 mobile Jest suites, and CI passed; the exit warning disappeared. |
| Ambiguous short knowledge links | PR #1212 made `internal/knowledge/lint.go` prefer exact full IDs and report ambiguous short names, with regression tests, OpenSpec, and bilingual CLI docs. | `go test ./internal/knowledge`, full `make check`, design check, strict OpenSpec validation, and six CI jobs passed before merge. |
| Local knowledge links | Reviewed source and target facts and sensitivity entry by entry, then used `gtmux knowledge add/supersede`; migrated seven old HQ notes and one archived lesson. Missing historical sources are labeled as such without invented links. | `knowledge lint --json`: entries 724 → 732; broken links 66 → 0; unmarked sensitive 1 → 0. These totals include the #1211 checker fix and local ledger edits. |

## Advisory findings and limits

- The 27 near-duplicate findings are candidates, not 27 confirmed duplicate facts. Review of titles and high-scoring bodies found distinct scopes, including the voucher workflow versus its guardian script, an assertion index versus an individual rule, and separate VPS hosts. No bulk merge or deletion was made. Use `neighbours` and `lint` when adding related entries.
- The 265 orphan findings mean no incoming or outgoing `[[links]]`. A standalone searchable fact can be valid. The checker documents this as advisory, so no artificial links were added to reduce the count.
- The original sensitive finding was explanatory `token = 新消息`, not a credential. The checker was corrected. Migrated HQ notes contain distilled lessons, not private conversation bodies. Sensitivity still needs review against each fact.
- Structured logs have size and retention limits and cannot guarantee a record for every external failure. This round makes append failures visible and audited actions traceable across the three stores. Diagnostics remain best effort. HQ distillation and self-checks distinguish request from completion with receipts; the quality of candidate processing still depends on HQ doing the work.
- Free disk space was about 12 GiB. `idevice_id -l` found no device, USB enumeration found no iPhone/iPad, and `devicectl` did not produce a device list. Native builds, installation, and physical-device screen reader/layout acceptance were therefore not performed. Device acceptance remains open until a device is connected; JS tests do not establish a physical-device result.

This follow-up did not cut a release or delete private knowledge or build caches.
