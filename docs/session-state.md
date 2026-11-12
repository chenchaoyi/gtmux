# Session status and duration

The Go core supplies the same session status to the CLI, Mac menu bar, mobile apps
and browser. A terminal repaint alone does not prove that a new task started.

When an old Codex active marker survives a later finished observation and its
frontend is unavailable, gtmux can still recognize a ready terminal with no work,
queued input, approval or observed CPU work. It displays idle while keeping all
lifecycle records intact. This correction does not send a completion notification
or label the row as newly completed. Fresh starts and verified frontends keep
their stricter identity and turn checks.

When real work is visible, a start timestamp from before a finished observation
cannot date that work. The current Working footer supplies elapsed time when
recognized; otherwise the duration is hidden. Status remains working. Older
servers keep their existing duration behavior.

Repeated status reads in the same second retain recently observed CPU work,
without extending its normal expiry. Clock rollback or a restarted CPU counter
does not provide new activity evidence.

Resume snapshots are published as complete atomic replacements. Codex hook saves
and frontend heartbeats share the binding lease, preventing a contended writer
from overwriting another binding. Unknown ownership and busy leases remain
conservative. This does not automatically repair damaged historical records or
restore a session.

See the [Chinese version](session-state.zh.md) and [API contract](../api/contract.md).

## Association pending

When current conversation metadata cannot be verified, a private historical binding may still identify readable history for the same verified terminal process. GTmux shows **Association pending · history read only** and folds that history into the pane instead of listing the same conversation again as external. Conflicting, ambiguous or missing process evidence does not authorize this fallback.

This observation is separate from the current session identity. It does not enable Chat sending, adoption, restore or lifecycle updates, change status or borrow a task timestamp. Use the Terminal view for existing manual controls. Guest views do not receive this fallback identity. Ordinary transcript clients retain their existing contract; the supported UI explicitly requests the read-only observation. A successful verified refresh removes the history restriction, including on an unchanged response. Original history files and live bindings are not rewritten.
