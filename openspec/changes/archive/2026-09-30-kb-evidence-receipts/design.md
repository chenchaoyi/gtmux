# Design

The ledger remains the authority. An add/supersede carries `sources` and a dismissal
carries the same snapshots plus its reason. `candidate_result` identifies accepted or
dismissed work. Reads subtract settled source IDs from the append-only candidate file.
A new observation with the same family key has a different ID and remains pending.

New candidates get an ID at append; transcript leads carry their existing stable miner
ID. Legacy candidates get deterministic read-time IDs from their canonical content and
occurrence number, without rewriting the operator's files. Digests cover the retained
candidate payload, not an assertion that its content is true. Missing original source
locations remain missing; mined correction leads additionally retain file/byte/turn
references when the reader supplies them.

A cgo-free advisory lock covers candidate append, settlement selection and ledger writes.
The ledger writer builds an unchanged prefix plus one JSON line in a same-directory temp
file, syncs it, closes it and renames it. Rename is the commit point; the previous file
survives every earlier failure. Directory-sync and render failures after commit explicitly
say the operation committed and preserve its ID. Retrying a resolved key refuses rather
than recording the same sources twice; a subsequent fresh candidate remains actionable.

All accepted/rejected snapshots remain in the ledger after supersede/retire. Folded
successors inherit the predecessor's source snapshots. Rendered topic files and carriers
do not include excerpts; only explicit source/detail and receipt reads return them.
Success/failure audit records share the operation ID and say their outcome and phase.
Audit writes remain best-effort under the existing write-health reporting contract;
the ledger receipt and command result remain authoritative if telemetry is unavailable.

The whole ledger is copied per write to avoid a partially appended record consuming
work. This adds O(ledger bytes) write I/O, not an in-memory copy of the archive. No
automatic archive deletion or retention policy is introduced in this batch.
