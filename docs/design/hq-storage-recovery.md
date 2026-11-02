# Storage and HQ recovery

Automatic recovery and full export have different scopes. Neither is evidence of an off-machine backup. This change does not migrate or delete existing archives.

## Automatic recovery

`gtmux config snapshots` shows only backup settings. `on|off`, `--keep`, `--max-mb` and `--interval-minutes` update that narrow setting without displaying the rest of the configuration. The default is on, three changed-content recovery points, a 64 MiB total managed budget and at least fifteen minutes between attempts. These are points, not a promise of three days. Closing automatic recovery preserves existing files.

The JSON key is `hqSnapshots`: `enabled`, `keep` (1–30), `maxMB` (1–2048), `intervalMinutes` (0–1440), and optional `include` containing exact relative file/directory paths. Malformed settings refuse writes/pruning. Missing settings use the shared defaults.

The core protects instructions/personal rules, all knowledge files, workspace records and ordinary notes, including unknown structured notes. Symlinks are not followed; operational hidden workspace files stay excluded. Under `notes`, images, video, PDF/archive packages and raw logs are process attachments. Named bulk relay JSON snapshots are process material. Under `notes/reports`, text conclusions and SUMMARY/FINAL/INDEX/OWNER/RETAINED/retention JSON are retained; other report payloads are process material. An exact `include` override protects a unique file or subtree even in this process scope. Unknown notes outside the declared process scope are never dropped just because they are large: an oversized unique item makes the backup fail.

New points live in `hq-snapshots/automatic-v1`, independently of legacy packages in the parent. A point contains ordinary tar/gzip data, accompanied by a private versioned receipt containing content digest, package SHA-256, size and validation time. No new private format is required to extract it. Full content and owner-mode digest, independent of mtime/date, prevents copying unchanged content across days. Distinct changes can retain separate same-day points.

Before publication, limits and the complete gzip/footer/archive/workspace schema are checked with the import reader. Both archive and receipt are durably published and rechecked before retiring anything. Only unchanged, fully verified points managed by this version are eligible. The keep count and compressed-byte budget apply after successful publication; status/receipt space is reserved. An in-progress package is additionally bounded by the smaller of the configured budget and import cap. Unknown, incomplete or corrupt files are protected, count against the available budget and can block a new point. An error preserves prior recovery points and is visible in the records/doctor projection. Concurrent attempts share the existing cross-process storage lock.

**The existing fourteen legacy packages are not adopted or pruned.** Changing the default, disabling backups, failed generation or finding a new valid core point cannot authorize deleting old full-history states. Their restore validity remains unknown until separately examined.

## Explicit full exports

`hq --export` and the owner HTTP export still include the complete records scope, including report payloads and attachments, rather than silently returning a core subset. Plain, passphrase-encrypted and streamed exports must satisfy the same existing import limits: compressed input and decompressed tar each at most 128 MiB, a file at most 32 MiB, and 5,000 entries. Large full exports now fail before replacing a destination or sending HTTP payload bytes. The existing import memory budget is not raised to accommodate large process archives.

Exports are staged privately, completely validated, then atomically published; the output must be outside the HQ home. A full archive over those limits needs an independently managed filesystem backup, or a future bounded streaming import design. This iteration does not claim to make the existing oversized full packages importable. Core restores retain the displaced original home, including process attachments that were not in the core point. Restored pending messages remain uncertain and never replay as new authorization.

## Stored deliverables

Build intermediates, downloaded test copies and one-off unpacked validation directories belong to task/build storage and end with the task. A public release channel is the primary source for reproducible release assets. Cross-task retention is exceptional: use an explicit delivery archive, separate from application runtime state, with `retention.json` describing purpose, source/build/channel, byte budget, minimum retention, review date and responsible reviewer. A review date does not authorize deletion. The schema is `delivery-retention.schema.json`; validate it with `scripts/check_delivery_retention.py`.

Retain unique signing/recovery material, required symbols and concise final acceptance evidence. Do not retain multiple package transports/unpacked validation copies by default. Do not copy private credentials into a manifest. Keep current/previous deliveries, active Store submissions/distributions and any uniquely required recovery evidence protected until their actual replacement and restore path have been verified. Older artifacts need an exact reviewed retirement plan; age, similar size, expiring CI artifacts or a new copy alone are insufficient.

An archive outside a worktree is not automatically permanent, and moving it to another directory on the same disk does not release space. No existing archive is moved by this change. Existing release, Store and reboot recovery materials retain their original retention authority.

## Storage presentation

The shared doctor projection reports runtime data, automatic backups (`hq-snapshots`), release recovery (`releases`, `appstore`), verification archives (`reports`, `reviews`, `verification`) and unclassified data separately. Unknown top-level groups are not described as runtime. Directory sources and the backup configuration entry accompany the totals. Retained archives do not become a runtime failure merely because they are large.

`hq --records --json` adds `automatic_backup`: the selected policy, managed/legacy sizes and counts, publication validation time and last failure. Legacy counts are not validated recovery counts. The Mac client consumes this projection, with a conservative fallback for older cores. Logical lengths, allocated directory bytes and concurrent volume availability are separate measurements; no GB release is claimed for this source-only mechanism change.

Tests use synthetic private homes: scope/full-export distinction, actual restore, legacy retention, budget/disable, same-content cross-day deduplication, concurrent publication, invalid/corrupt/oversized input, existing-destination preservation and storage classification. They never import into the real HQ home or inspect private desktop conversations.
