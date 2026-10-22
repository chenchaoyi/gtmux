## Why

A Codex TUI launched by HQ can retain the previous conversation's resume binding when same-directory peers make hook cwd attribution ambiguous. Its hook observation then appears as a second, external conversation even though the actual frontend remains in the pane.

## What changes

Add a narrow process-backed recovery path. Reuse the verified foreground terminal inspector (stable process chain and birth, foreground group, actual character-device stdin, fresh pane identity). Combine that evidence with a same-home, explicit CLI rollout identity and exact hook process incarnation. Fresh main-session start/submit hooks can identify their own pane; existing observations may reconcile only when exactly one eligible session claims the client. Retain nonce delivery proofs, refuse ambiguity and preserve all real history. Do not infer ownership from title, cwd, inherited pane, backend PID alone or creation metadata alone.

## Surfaces

- Terminal: restore the correct resumable identity for a verified live Codex TUI, preserving ordinary send guards.
- Menu bar: one managed pane row rather than an external duplicate; uses the shared core.
- Phone: existing pane chat and input use the corrected shared binding; no new controls.
- iPad: same shared contract as Phone.
- Web: same managed pane identity; existing authentication, consent and input limits remain.

## Validation and limits

Synthetic coverage must reject shared/headless backends, piped stdin, PID reuse, changed panes/records, multiple session claims, other homes and non-CLI sessions. Exercise stale incumbent recovery, hook precedence and deduplication without deleting observations. The actual reported instance is read-only evidence; no user-session input, restart or configuration change is part of validation. Independent review and physical-device acceptance are separate.
