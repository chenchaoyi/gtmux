# Storage and HQ recovery

Automatic recovery and full export have different scopes. Neither is evidence of an off-machine backup. This change does not migrate or delete existing archives.

## Automatic recovery

`gtmux config snapshots` shows only backup settings. `on|off`, `--keep`, `--max-mb` and `--interval-minutes` update that narrow setting without displaying the rest of the configuration. The default is on, three changed-content recovery points, a 64 MiB total managed budget and at least fifteen minutes between attempts. These are points, not a promise of three days. Closing automatic recovery preserves existing files.

The JSON key is `hqSnapshots`: `enabled`, `keep` (1–30), `maxMB` (1–2048), `intervalMinutes` (0–1440), and optional `include` containing exact relative file/directory paths. Malformed settings refuse writes/pruning. Missing settings use the shared defaults.

The core protects instructions/personal rules, all knowledge files, workspace records and ordinary notes, including unknown structured notes. Symlinks are not followed; operational hidden workspace files stay excluded. Under `notes`, images, video, PDF/archive packages and raw logs are process attachments. Named bulk relay JSON snapshots are process material, including generated `*-relays-final.json`, `*-relays-verified.json`, `*-relays-closeout.json`, `*-relays-closure.json` and other observation-phase variants. Summary/analysis JSON notes outside this explicit pattern remain protected. Under `notes/reports`, text conclusions and SUMMARY/FINAL/INDEX/OWNER/RETAINED/retention JSON are retained; other report payloads are process material. An exact `include` override protects a unique file or subtree even in this process scope. Unknown notes outside the declared process scope are never dropped just because they are large: an oversized unique item makes the backup fail.

New points live in `hq-snapshots/automatic-v1`, independently of legacy packages in the parent. A point contains ordinary tar/gzip data, accompanied by a private versioned receipt containing content digest, package SHA-256, size and validation time. No new private format is required to extract it. Full content and owner-mode digest, independent of mtime/date, prevents copying unchanged content across days. Distinct changes can retain separate same-day points.

Before publication, limits and the complete gzip/footer/archive/workspace schema are checked with the import reader. Both archive and receipt are durably published and rechecked before retiring anything. Only unchanged, fully verified points managed by this version are eligible. The keep count and compressed-byte budget apply after successful publication; status/receipt space is reserved. An in-progress package is additionally bounded by the smaller of the configured budget and import cap. Unknown, incomplete or corrupt files are protected, count against the available budget and can block a new point. An error preserves prior recovery points and is visible in the records/doctor projection. Concurrent attempts share the existing cross-process storage lock.

**The existing fourteen legacy packages are not adopted or pruned.** Changing the default, disabling backups, failed generation or finding a new valid core point cannot authorize deleting old full-history states. Their restore validity remains unknown until separately examined.

## Explicit full exports

`hq --export` and the owner HTTP export still include the complete records scope, including report payloads and attachments, rather than silently returning a core subset. Plain, passphrase-encrypted and streamed exports must satisfy the same existing import limits: compressed input and decompressed tar each at most 128 MiB, a file at most 32 MiB, and 10,000 entries. Large full exports now fail before replacing a destination or sending HTTP payload bytes. Neither compressed/decompressed byte limit nor the per-file limit is raised. Record buffers use the exact declared size, avoiding a minimum buffer per tiny record and geometric over-allocation. The separate entry cap is 10,000; earlier readers limited to 5,000 refuse larger points, so restore those points with 1.0.168 or newer.

Exports are staged privately, completely validated, then atomically published; the output must be outside the HQ home. A full archive over those limits needs an independently managed filesystem backup, or a future bounded streaming import design. This iteration does not claim to make the existing oversized full packages importable. Core restores retain the displaced original home, including process attachments that were not in the core point. Restored pending messages remain uncertain and never replay as new authorization.

## Stored deliverables

The default gtmux configuration and data trees (`~/.config/gtmux/` and
`~/.local/share/gtmux/`) are reserved for gtmux-managed configuration, runtime
state and its own archives. They are not a general project workspace or delivery
store, even when developing gtmux itself. Project code, scripts, documents and
deliveries belong to project-owned storage; builds, fixtures, downloads and raw
evidence belong to task temporary storage. A retained delivery outside a worktree
uses a project-owned archive with the policy below, not a new folder in gtmux's
application data. HQ notes keep concise coordination summaries and references to
the evidence rather than another copy of project payloads. Only reusable HQ
knowledge tools belong in `knowledge/tools/`.

This boundary is built into every agent's managed global instruction block and
HQ playbook version 60. Knowledge sync preserves user text and hand-edited block
protection; normal HQ upgrades preserve LOCAL.md and existing records. The rule
does not migrate or delete old material. Existing `reports`, `reviews` and release
groups remain visible in storage accounting; their presence does not authorize
new general-purpose project storage there. User-facing report links use the
project delivery path: the protected configuration tree is not remotely readable.

Build intermediates, downloaded test copies and one-off unpacked validation directories belong to task/build storage and end with the task. A public release channel is the primary source for reproducible release assets. Cross-task retention is exceptional: use an explicit delivery archive, separate from application runtime state, with `retention.json` describing purpose, source/build/channel, byte budget, minimum retention, review date and responsible reviewer. A review date does not authorize deletion. The schema is `delivery-retention.schema.json`; validate it with `scripts/check_delivery_retention.py`.

Choose the project's existing private delivery root before copying a permanent archive. Use one directory per release version or Store version/build. Check the destination first, then check the same project boundary and retention budget at handoff.

```sh
# Set PROJECT_DELIVERY_ROOT to the project's existing delivery directory.
delivery="$PROJECT_DELIVERY_ROOT/releases/vX.Y.Z"
python3 scripts/check_delivery_retention.py --project-root "$PROJECT_DELIVERY_ROOT" --destination "$delivery"
# After retaining the selected files and writing retention.json:
python3 scripts/check_delivery_retention.py --project-root "$PROJECT_DELIVERY_ROOT" "$delivery/retention.json"
```

The destination check rejects directories outside the project root and paths or aliases into either gtmux-managed tree. It checks the current path without creating it. An existing manifest can still be inspected without the project-root option when reviewing a legacy archive.

Mac release staging, encrypted mobile delivery retrieval, Store package verification receipts and Store capture/render tooling apply the same managed-tree check before their first write. Outputs for temporary work can live in a private task directory. Use the project-root check for the permanent handoff.

Retain unique signing/recovery material, required symbols and concise final acceptance evidence. Do not retain multiple package transports/unpacked validation copies by default. Do not copy private credentials into a manifest. Keep current/previous deliveries, active Store submissions/distributions and any uniquely required recovery evidence protected until their actual replacement and restore path have been verified. Older artifacts need an exact reviewed retirement plan; age, similar size, expiring CI artifacts or a new copy alone are insufficient.

An archive outside a worktree is not automatically permanent, and moving it to another directory on the same disk does not release space. No existing archive is moved by this change. Existing release, Store and reboot recovery materials retain their original retention authority.

## Storage presentation

The shared doctor projection reports runtime data, automatic backups (`hq-snapshots`), release recovery (`releases`, `appstore`), verification archives (`reports`, `reviews`, `verification`) and unclassified data separately. Unknown top-level groups are not described as runtime. Directory sources and the backup configuration entry accompany the totals. Retained archives do not become a runtime failure merely because they are large.

`hq --records --json` adds `automatic_backup`: the selected policy, managed/legacy sizes and counts, publication validation time and last failure. Legacy counts are not validated recovery counts. The Mac client consumes this projection, with a conservative fallback for older cores. Logical lengths, allocated directory bytes and concurrent volume availability are separate measurements; no GB release is claimed for this source-only mechanism change.

Tests use synthetic private homes: scope/full-export distinction, actual restore, legacy retention, budget/disable, same-content cross-day deduplication, concurrent publication, invalid/corrupt/oversized input, existing-destination preservation and storage classification. They never import into the real HQ home or inspect private desktop conversations.

## Historical process material and doctor

Doctor inventories only recognizable project material under HQ notes, separating
raw relay exports, declared report/attachment payloads and source files. Unknown
archives outside declared project reports are retained and need ownership/recovery
review. Knowledge, workspace, instructions, unknown ordinary notes and existing
recovery points are not cleanup candidates. This row and doctor fix never move or
delete material. A separately authorized cleanup uses project-owned storage, exact
source checks and full archive/SHA readback before removing redundant plaintext;
HQ keeps only a concise index. A same-volume move is not disk space recovered.

Legacy relay-like JSON names are excluded from automatic core recovery only when
a complete, nonempty, bounded relay-request array confirms they are observations.
IDs follow the owning relay contract, including custom IDs. Completeness requires
the real Put source rule (pane or Codex home/thread), every mandatory serialized
key and known typed nested structures. Unknown extensions, duplicate keys, nulls
and partial structures are retained; formatting and key order are irrelevant.
Malformed, empty or
unknown notes remain protected. Existing generated-name exclusions, explicit
include overrides and full-export scope retain their meanings. This recognition
never deletes the source or changes the128MiB compressed/decompressed import cap,
32MiB file cap or10,000-entry cap. The128MiB cap bounds a validated recovery
package, not the entire HQ directory; the default64MiB is a separate total managed
compressed snapshot budget. Raising the cap is not a substitute for classifying
project attachments. A new point or a status timestamp does not prove a real
restore, nor authorize retiring older unique recovery evidence.
