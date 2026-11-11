# Conversation association recovery

[中文](chat-association.zh.md)

A pane can still run while its conversation index is unreadable. This is an association failure, not proof that the conversation was deleted. Chat and Trace share the pane reader; the CLI's explicit verified history reader is independent.

The primary index remains strict JSON. A missing or malformed index may use one independently stored live Codex frontend receipt for **read-only history**. The exceptional lookup reads at most 128 entries, rejects malformed/ambiguous/newer conflicting evidence, and rechecks home, location, client/root/backend process births and TTY. It never trusts a JSON prefix, title, cwd similarity, launch arguments or a previous checkpoint as the current conversation. `LoadLive`, input routing and restore retain their original authority rules.

The current authenticated bridge can repair its own missing or malformed primary index on its normal heartbeat. The existing current-epoch check and binding lease remain; repair requires its exact persisted fresh receipt and repeated process checks. A valid incumbent or changed receipt prevents repair. Reads alone do not write, and no user session is restarted or restored.

The HTTP reader reports `conversation-unassociated` (409) or `conversation-unavailable` (503); valid empty history remains `[]`. App and browser keep already displayed messages and show Retry with association guidance. Successful recovery carries `X-Gtmux-Conversation-State: recovered`, without adding input permission.

If both the primary index and the independent receipt are unavailable, automatic association is intentionally unavailable. Existing history can still be opened by its independently verified conversation identity. A future successful authenticated root-thread binding can establish a new association. Do not manually trim corrupt JSON or clear frontend fields to manufacture a legacy binding.

Validation uses isolated fixtures and process seams. It does not certify the currently running release, real TTY routing, mobile rendering, or live session repair. Reviewed cases: CAS-001 through CAS-005.
