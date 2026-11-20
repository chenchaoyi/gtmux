# Shared Go and DSH input store gate

When the cutover record and new path are both absent, Go and DSH may open a valid v1 queue or create one under a clean owner-controlled state root. An old path occupied by a marker, link or incompatible object cannot be recreated as a queue.

At `active`, `terminal-input-cutover.json` binds the device and inode of `terminal-input-v2`, the device and inode of the regular marker at the old path, and their common generation. Both metadata files must be single-link, private regular files owned by the current user. Missing, invalid, prepared, rolling-back or inconsistent evidence leaves no writable cutover path.

Go binds preparation, claiming and settlement to the validated layout and directory identities. The binding lives in memory. Stored v1 records retain their existing format, and looking up a settled receipt grants no new submission authority. A claimed ID without a trustworthy receipt remains protected from replay. DSH checks the chosen layout at registration and checks again before reporting readiness or submitting. On an active layout Go requires DSH readiness for the same generation.

`agentclient.InputDir` is a read-only layout check. It neither creates a queue nor acquires a lock. Its result does not establish that the old processes have drained.

| Layout | Current Go and DSH | Old Go and DSH |
| --- | --- | --- |
| Valid v1 outside maintenance | Use the old directory. | Existing layout is compatible. |
| Cutover prepared, rolling back or inconclusive | Decline write authority. | Must be paused and drained before exchange. The marker blocks new old-path opens after exchange. |
| Verified active generation | Reopen the new directory on restart and require matching DSH readiness. | The old pathname cannot open the new queue. |

An operator coordinates entrance closure, settlement and all old descriptor release before preparing or exchanging paths. The product gate chooses or rejects a layout. It does not write an activation record or roll it back.

The incident fix permits 8,192 effective slots and keeps the independent 8,192-entry scan bound. v1.0.186 required 4,096 effective slots, which could reject a preserved legacy backlog before any new input was submitted. The 6,144-slot measurement was for a fixture with 3,168 physical names and 1,347 reservations. A transition setting would require a distinct scan ceiling, measurement of serialized and allocated bytes for records and temporary writes, and enough headroom for settlement. The 1 MiB body limit is separate from these costs. See the [maintenance steps](input-store-maintenance.md) for the read-only checks and activation responsibilities.
