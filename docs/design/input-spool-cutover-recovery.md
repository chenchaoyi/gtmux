# Input queue drain and recovery

This extends the [cutover design](input-spool-cutover.md). Old Go pathname operations fail once a regular-file marker occupies the old path. An already open file or directory descriptor can still write to the original inode. Production keeps the v1 file schema and 4,096-slot default.

## Drain and activate

1. Stop new submissions to old Go and DSH. Return a recognizable maintenance result and retain unresolved client operations under their original IDs.
2. Let tracked writers finish receipts, close their descriptors and exit. Confirm that automated restarts and all admission paths are paused. A handle listing supports this check. The new lease cannot stop old receipts or old DSH.
3. Bind both objects by device, inode, type and owner in a durable cutover record. Exchange the directory and marker atomically. New Go and DSH inspect the same record. A DSH receiver already registered must recheck before declaring readiness or executing a command.
4. Enable the new layout only after the activation record is durable. Preserve both objects and stop admission if the observed stage or identity is uncertain.

| Durable stage | Observed paths | Recovery |
| --- | --- | --- |
| Prepared | Queue and marker still at their original paths | Cancel preparation and verify v1. |
| Prepared | Queue and marker exchanged | Reverse the exchange only with all writers stopped and no activation. |
| Rolling back | v1 restored, marker present or already removed | Finish cleanup of this transaction's marker and record and verify v1. |
| Active | Original queue now at the new path, bound marker at the old path | Restart on the active generation and preserve all new receipts and uncertainty. |
| Missing, malformed or inconsistent evidence | Any | Keep writes paused and preserve both paths for reconciliation. |

An active generation cannot automatically restore authority to old writers. A downgrade requires a new drain, receipt reconciliation, and verification of old capacity, deduplication and receipt behavior.

## Evidence and limits

- In the macOS fixture, both an open directory descriptor and an open regular file remained writable after the exchange. The drain signal waited until the tracked writer had flushed its receipt, closed both descriptors and exited.
- Interrupted private transactions before exchange, after exchange and before activation, and during rollback recovered the original inode and v1 bytes. After activation, restart preserved the new receipt and disallowed automatic rollback. Missing or mismatched evidence failed closed.
- The DSH fixture exercised registration, v1 restoration, same-ID submission on the active layout and rejection after identity drift. It used synthetic agent and tmux inputs. The initial run failed to register on v1 restoration for an undetermined reason. A new private environment passed without a source change.
- The 6,144-slot trial provided 1,629 spare effective slots for the 4,515-slot sample. It also raised the derived scan from 8,192 to 12,288, while the sample contained only 3,168 physical names. Keep the production admission default and an independent scan bound. A future transition allowance needs a bounded owner-only setting, an allocated-byte budget and restart/failure coverage.

See the [shared startup gate](input-store-gate.md) for the production candidate. Actual service order, process inventory and hardware power-loss recovery require separate verification.
