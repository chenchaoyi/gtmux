# Terminal input store maintenance and read-only preflight

The production candidate reads the v1 layout or selects `terminal-input-v2` after the active record, marker and data directory identities agree. Go binds the verified directory and generation in memory. DSH checks the generation on registration, readiness and execution. Prepared, rolling-back and inconclusive states refuse writes. The candidate does not provide a production activation tool or a switch that stops all admission. See the [startup gate](input-store-gate.md) and [interruption evidence](input-spool-cutover-recovery.md).

## Observe the instance

Run the preflight as the owner of the instance so that file ownership is checked against the right user.

```sh
state_root="$HOME/.local/share/gtmux"
python3 -B docs/design/input-store-preflight.py "$state_root"
df -k "$state_root"
command -v gtmux
gtmux --version
```

The script reads layout and ownership metadata, name classes, logical and allocated sizes, and DSH readiness. Its output omits bodies, request IDs and the generation token. Exit 0 with `legacy`, `fresh` or `active` establishes the observed layout only. `maintenance_ready` remains `unverified`. Exit 2 or `unverified` requires investigation. Count settlement reservations and reconcile v1 indexes, claims and receipts separately. A live directory can change between samples.

Inventory this user's `com.gtmux.serve`, `com.gtmux.selftunnel` and `com.gtmux.tunnel` launchd jobs plus related processes. Retain job status, PID, start time, executable source and instance ownership. For known processes, use bounded listener and descriptor inspection, such as `lsof -nP -a -p <PID> -iTCP -sTCP:LISTEN`. Include both paths, the marker, leases, temporary files and unlinked handles. Record unavailable data, timeouts and permission errors separately. Filter service arguments before retaining evidence. A binary or plugin hash on disk does not identify what a running process loaded.

| Ingress | What must be accounted for |
| --- | --- |
| Local CLI, menu bar and automation | `gtmux send`, `spawn` initial input, screenshot send, HQ workspace send and drive, relay delivery and automated wake. Include schedules and copies of old executables still runnable. |
| Phone, browser, tunnel and local HTTP | `POST /api/send`, paired and shared clients, and access that bypasses tunnels. Inspect existing writable `/api/attach` sockets and raw terminal input that can invoke DSH. `GET /api/interaction` only observes state. |
| Loaded DSH receivers | Default `~/.dsh/gtmux/terminal-dst/bridge.mjs`, separate `DSH_HOME` roots, loaded `/gtmux-send` commands, direct terminal access and restart paths. Preserve settlement while stopping entry of new commands. |

## Maintenance order

1. The coordinator records the exact instance, verified Go and DSH versions, owners of every input path, disk headroom, the maintenance window, recovery operator and independent verifier. Report Data below 8 GiB before starting the next high-write check. Let checks already running finish.
2. The ingress owners stop new sends and stop automatic restart of old receivers. Retain confirmation and timing for each input path and an inventory of in-flight deliveries. Check existing sockets, automation and local clients. Let old work settle and verify the old writers have exited or irrevocably released relevant intake and descriptors. A transient empty handle listing, a sleep or the new lease is insufficient.
3. Reconcile by original ID and target. Retain accepted receipts for deduplication, pending input, and the original index and claim for any unresolved outcome. If a handler is still active, keep waiting. Record the original device and inode and keep approved recovery copies or proofs. The legacy writers used atomic rename without explicit sync. The operator must establish file and directory durability with the approved tool after writes stop, then verify actual restart and power-loss behavior separately. Preserve data and keep intake closed on a sync or reconciliation failure.
4. The approved tool and designated operator persist the prepared record, exchange the two objects on the same filesystem, verify the original inode at the new location and the single-link marker at the old one, sync the exchange and parent directory, then persist and read back the active record. Verify old writers remain paused. Isolated tests showed that the marker blocks new old-path opens, while descriptors opened before the exchange remain writable.
5. With intake still paused, the service owner starts the verified Go and DSH versions. Only open DSH ingress after same-generation readiness is established. Use approved verification for the installed process versions, identities, stable IDs, receipts and rejection at the old path. The independent verifier checks that pending, unresolved and accepted records are accounted for before the coordinator restores each input path.

| Observed after interruption | Recovery responsibility |
| --- | --- |
| Prepared or rolling back | Keep admission stopped. The recovery operator confirms that the new generation never opened for writing, then follows the documented continuation or rollback steps using the bound identities. Verify the original inode, v1 records and receipts before restarting the original version. |
| Active or new writes possible | Recover forward on the active generation while keeping admission paused. Recheck the record, marker, queue, receipts and Go/DSH readiness. A downgrade needs a distinct drain, compatibility and deduplication review. |
| Missing or inconsistent evidence, or unknown outcome | Keep both paths and original logs. The recovery operator reconciles the maintenance material and the independent verifier confirms before the coordinator reopens any input. |

## Capacity

The default remains 4,096 effective slots and the scan bound is 8,192 names. The 4,515-slot fixture consisted of 3,168 names and 1,347 reservations, and admitted new input and settled old input at 6,144. The fixture also changed a derived scan bound, so it cannot justify raising the production scan. A bounded transition setting requires an owner-only scope, expiry, an independent scan budget and a byte budget for serialized and allocated records, claims, receipts and writes in progress. The 1 MiB maximum body size cannot substitute for that budget.

Production use requires actual stop-and-drain coordination, verified activation and recovery tooling, service restart and fault validation, independent review, normal CI and release gates.

## Retired send IDs

The complete receipt stays available for at least 72 hours. On retirement, accepted stable IDs move to exact 16-byte keys in the private `.retired` directory. Prefixes share a 64 MiB budget for committed keys and can allocate additional 256 KiB pages within that budget. This is separate from the 4,096 active slots and 8,192-entry scan bound. The last 128 KiB of key capacity is reserved for already admitted records.

Run `gtmux doctor --no-prompt` to check archive occupancy, integrity and recovery state. At 80 percent, plan capacity and take a consistent backup. At the settlement reserve, admission pauses for new stable IDs. Complete admitted operations normally and keep their receipts. Continued stable admission requires a capacity increase in a supported version or a migration that preserves every accepted key. Deleting pages cannot supply safe capacity.

Keep the active queue, `.retired-catalog.json` and `.retired/` together in the same consistent snapshot. The catalogue records committed lengths and digests. If a page is missing or damaged, the guard withholds authority for unverifiable IDs in that prefix. Healthy prefixes can continue when active slots remain. If the catalogue itself is unavailable, stable-ID lookup fails closed.

During an authorized maintenance window, stop new intake and drain writers before restoring exact pages that match the surviving catalogue. If the catalogue is also damaged, verify candidate backups in isolation and reconcile changes since the snapshot before reopening. Preserve the affected files and the checkpoint. A recognized pending checkpoint can resume on admission. A damaged checkpoint needs operator reconciliation. Capture the doctor verdict before and after recovery. An empty archive cannot establish that old sends never happened.
