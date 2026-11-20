# Shared input queue cutover and capacity

gtmux v1.0.185 uses the v1 JSON files in `terminal-input/`. Its Go sender could admit another request when the directory already had 4,096 entries, and its receipt writer does not take the new settlement lease. An empty handle sample or a lease held by the new process does not establish that old writes have drained.

The read-only inventory counted 3,168 names and at least 1,347 settlement reservations, for at least 4,515 effective slots. Preserve pending requests, unknown outcomes, and receipts within retention.

## Cutover design

1. Stop admission to every old Go and DSH entry point. Allow in-flight work to settle, verify its receipts and close all old directory, temporary-file and write handles. Preserve any unresolved claims and disallow replay.
2. Prepare a private regular-file marker in the same managed parent. Verify both objects, their ownership and device, and rule out symlinks. With writers stopped, exchange the directory and marker atomically, moving the original directory inode to `terminal-input-v2`. Bind the layout and object identities in durable records. If the filesystem cannot exchange these objects or their identity is uncertain, keep writes paused.
3. Start verified new Go and DSH writers after the activation record is durable. Old Go and DSH use the old pathname, where the marker blocks new opens. The new writers check the same layout generation for admission, claim, settlement, lookup and cleanup.
4. Size any transition allowance against the actual inventory and allocated bytes. The fixture used 6,144 slots. Production still defaults to 4,096 with a separately bounded directory scan. A prospective owner-only override would apply only to the new layout. Existing v1 JSON limits and the 1 MiB request body limit still apply. Measure the entire queue independently of its per-request size.

## Evidence

A macOS private fixture kept the same directory inode and v1 file bytes after the exchange. The fixed v1.0.185 Go `Prepare`, `ClaimNative` and `FinishNative` entries failed through the old pathname. A separate 4,515-slot fixture admitted a new ID and settled an original pending request under the 6,144 trial limit, while retaining pending, uncertain, deduplication and accepted-receipt evidence. Contradictory settlement was rejected.

Production still requires coordination of all admission paths, old descriptors and loaded DSH writers, explicit recovery ownership and normal release gates. Any downgrade after the new writers may have committed requires a separate drain and reconciliation.
