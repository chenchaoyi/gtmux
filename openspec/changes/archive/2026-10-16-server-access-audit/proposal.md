## Why
The server retains enrolled-device last-seen state and some action diagnostics, but an owner cannot inspect a coherent history of external readers and operations. Permission checks currently omit foreign-owned/unreadable paths and describe only read exposure.

## What changes
- Record verified device/link identity, client metadata, trusted source address, time, allowlisted operation, optional pane locator and HTTP outcome. Record every mutation and stream lifecycle; coalesce successful reads for five minutes per identity/operation/pane. No payload, authorization value, query string, image or conversation text.
- Reuse the private diagnostic store and existing retention (30 days / 100 MiB by default). Provide a bounded, paginated access-only reader and owner-only endpoint. The CLI and native/app views consume the same history model.
- Harden diagnostic append files against symlinks/foreign ownership and unsafe existing modes. Doctor reports mode exposure, unverified ownership, unreadable paths and relevant unsafe links; repairs narrow only owned paths and never follow links into unrelated data.

## Surfaces
- CLI (terminal): logs --access offers human and paginated JSON history.
- Menu bar (menubar): Preferences pairing section opens a read-only access-history window.
- phone: owner server management exposes a read-only history with refresh/load earlier states.
- iPad: same history source and bounded adaptive content column.
- Web: owner-only API available; no new browser viewer in this change. Guest access denied.

## Boundaries
HTTP success confirms a request was accepted, not eventual agent completion. Master-token identity cannot identify an individual device; names/platform are client metadata, not authentication factors. POSIX modes protect against other local accounts, not the file owner or root, and do not attest ACLs or tamper-proof storage. Logging failure remains visible through stderr/write-health checks; it does not turn normal session operations into fail-closed controls. History begins after upgrade; no old activity is invented.
