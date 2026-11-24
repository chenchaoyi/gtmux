# gtmux state storage lifecycle

[中文](state-storage-lifecycle.zh.md)

The gtmux configuration and data roots are `~/.config/gtmux/` and
`~/.local/share/gtmux/`. Their paths are part of the interface shared by CLI
and agent hooks, the local server, desktop helpers and recovery tools. Keep
project sources and deliveries in project-owned storage and raw build outputs
and evidence in task temporary storage. HQ can reference those results from its
coordination notes. See the [path contract](../../internal/state/state.go) and
[delivery retention policy](hq-storage-recovery.md#stored-deliverables).

## What each store protects

| Existing store | Writer and reader | Retention and recovery |
| --- | --- | --- |
| Configuration, knowledge, HQ records, device permissions and external-control consent | Configuration and HQ operations write them. Serving, restore and authorization paths read them. | Preserve current records and proven recovery points. Changing authorization or dropping a record needs its owning operation. Restore into an isolated destination before trusting a replacement. |
| Serve/agent status, pane markers, client registration, socket links and active coordination locks | Hooks, agents, the service and desktop helpers publish them. Radar, resume and maintenance probes consume them. | The owning process manages expiry and regeneration. Preserve locks while writers may still open them and inspect links and sockets without following them in size or cleanup scans. [Private storage writers](private-storage.md) |
| `participant-profiles/` | Hooks record observations. Profile readers merge old snapshots with observations in `v2/`. | Keep both generations of JSON while the compatibility reader uses them. The `v2/locks/` directory lazily uses up to 256 lock paths. Any number of profiles can share those paths. A hook skips that best-effort observation if another writer holds its shard, then continues its regular work. [Profile writer](../../internal/agentprofile/observe.go) |
| `terminal-input/`, `relay/`, HQ reminder journals, session restore and replay records | Producers, receivers and reconcilers share request IDs, claims, receipts and cursors. | Preserve pending, held, unknown, inconsistent and retriable records. A receipt, a live inbox item and evidence of application are separate. Compaction needs a verifiable terminal outcome, recovery readback and continued lookup by the original ID. [Input recovery](input-spool-cutover-recovery.md) |
| Upload staging, logs and rendered caches | HTTP/CLI writers and UI helpers produce them. Input delivery, diagnosis and interface rendering consume their respective data. | Apply the owner's existing size/time rules. Published attachment copies under `artifacts/` have reference and revocation semantics. An image cache can rebuild, while a particular failure log or accepted attachment can be unique. [Upload/log maintenance](../../internal/hq/diskhygiene.go) · [Attachment store](../../internal/artifact/store.go) |
| `hq-snapshots/` and legacy release, store, report and review material | HQ owns its recovery points. Project maintainers own their release and verification material. | Managed points retire under the backup policy after a replacement is published and verified. Legacy recovery packages and project material need owner and restore review before any retirement. [HQ recovery](hq-storage-recovery.md) |

Doctor's storage totals use logical byte counts. For a disk audit, record
logical length, allocated bytes and path counts separately, deduplicate hard
links by device and inode, and record permission errors and paths that change
during a scan. APFS shares capacity across volumes. A before/after free-space
sample includes other processes' writes and changes in APFS allocation.
Keep unclassified data visible until the reader, writer and restore implications
are known. [Doctor storage projection](../../internal/app/doctor_storage.go)

## Layout for future stores

Use the following boundaries during design. Existing path contracts continue to
apply until an individual store completes its migration.

- Put settings, policies and durable HQ records in the managed configuration
  tree. Keep credentials with their existing authorized stores and backups.
- Give each long-lived subsystem a stable directory in managed data. Put schema
  generations beneath the subsystem when readers need both. For example,
  `participant-profiles/v2/` separates current JSON and its bounded `locks/`
  from older observations. Keep journals, ephemeral coordination and recovery
  points in separate subdirectories where one subsystem owns all three.
- Name records using their existing durable identity, such as a request ID or
  schema generation. Keep a bounded pool of lock paths for high-cardinality
  stores. Rotate completed aggregates only after compatibility and recovery
  checks. Use a single owner for each directory's policy rather than a shared
  drop folder.
- Put retained project deliverables under a project-owned release/version or
  task directory. Write a retention manifest at handoff with the exact source,
  size budget, review owner and date. Keep the current and previous release
  recoverable while the replacement is being verified. Validate a copy and
  recovery in another destination before retiring an original.
  [Retention manifest and checks](hq-storage-recovery.md#stored-deliverables)

## Inspection, cleanup and migration

`gtmux doctor --json` provides a read-only report. For the empty old participant
lock files, first upgrade or retire every old hook writer and let in-flight
hooks finish. Then run `gtmux doctor --prune-profile-locks` and confirm in its
interactive prompt. The check covers the exact old name and file shape, uses a
nonblocking lock and rechecks identity before removal. It can report a held file
as kept. Use doctor again to verify the count and service health. Preserve
current shard lock names even when idle so concurrent openers continue to use the
same inode. [CLI documentation](../cli.md) · [Cleanup implementation](../../internal/state/profilelocks.go)

For a future migration or extra deletion, inventory the people and actual
binaries that write/read the paths. Name the exact source, destination and scope
before seeking authorization. Check ownership, device/inode, live handles,
pending work, dependencies and recovery requirements. Stage and verify the
copy with checksums and an isolated restore before changing routing. For shared
queue or lock paths, stop admission, drain and verify old writers, publish the
generation durably, then test mixed-version behavior, crash recovery and
rollback. Keep unknown files and duplicate candidates pending until ownership
and the recovery path are established.
[Input spool migration and recovery](input-spool-cutover-recovery.md)

## Grouped data layout

`gtmux doctor --storage-layout` lists known stores, their grouped paths and any inactive copy. The selected layout lives on disk. Existing installations with a flat root select flat routing. `layout-v2/` has these groups.

| Group | Contents |
| --- | --- |
| `panes/` | Turn state, busy/idle samples, goal and pane appearance markers |
| `clients/` | Native observations and capture requests |
| `service/` | Service maintenance, resource and tunnel observation markers |
| `sessions/` | Resume and restore records, profiles, conversation observations and external consent |
| `usage/` | Usage counters, models and limits |
| `hq/` | HQ wake queues, claims, recovery evidence and maintenance |
| `inbox/` | Relay and dispatch records |
| `events/` | Current and rotated events with their sequence counter |
| `backups/` | HQ snapshots and export receipts |

The root path, desktop and launchd entry points, logs and icon cache, socket aliases, shell shims, input spool and terminal-client registration, global lock anchors and attachment URLs have their existing names. Unknown data and old project archives retain their original paths. The layout registry is the common Go routing table for moved stores. Native mobile and Web use the existing service contracts; the Mac app keeps its existing shared paths.

### Offline transition

Use a standalone shell. Stop admissions and let producers finish, then exit gtmux, tmux and the menu bar app, disable automatic relaunch and pause hooks that might invoke an older binary. `gtmux doctor --group-storage` asks for confirmation and checks for remaining local gtmux and tmux processes. Unload the managed launchd jobs and verify persistent disable for installed LaunchAgents in both domains. For systemd, stop and disable the user units, including any pending restart or start job. A manager that cannot be verified stops the move. Custom services, renamed programs and older hooks require operator review. Compatible commands hold shared admission for their lifetime. The transition takes exclusive admission, records the plan, rechecks writers and verifies device, inode, owner and file type before exclusive same-volume renames. It syncs the folders before switching routing.

An interruption keeps normal commands gated. Inspect `--storage-layout` for moved and pending stores. If inspection confirms all identities, repeat the same operation after verifying that writers are stopped. Recovery accepts exactly one location with the recorded identity for each entry. An occupied destination, incomplete evidence, symlink or foreign-owned object stops the operation. A flat store recreated by a legacy writer during grouped operation is surfaced as a conflict. The grouped installation requires compatible CLI and hooks.

Before a binary downgrade, use the same maintenance procedure and `gtmux doctor --flatten-storage`. This returns all current grouped records to their flat paths, including records written since grouping. It rejects conflicting copies. If an interrupted atomic write left private staging files at the group level, rollback saves their original bytes in `layout-recovery/<id>/`. Those files never become live state. Unrecognized entries still require review. No state migration runs automatically after installation.
