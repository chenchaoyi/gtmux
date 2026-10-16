# Server access history

## Purpose and entry points
An owner can review which paired device or browser accessed this server, when, which operation was requested and its HTTP result. This is a request audit, not a conversation archive or an agent-completion log.

- Terminal: `gtmux logs --access`; add `--json` for a paginated object, `--before <cursor>` for an earlier page, `--limit 1..200`, `--since 3d`, or `--actor <full-device-id>`.
- Menu bar: Preferences → paired devices → Access history. The local window reads the CLI projection.
- Phone and iPad: Settings → Manage server → Access history. The shared screen has refresh, earlier pages and explicit unsupported-server/permission/expired-page states. Guests cannot open this view or query its API.
- Web: requests are audited; an owner API is available. A browser history viewer is not included in this change.

## Record and privacy boundaries
The diagnostic store holds informational `serve/access.request` action records. Each contains an operation ID, timestamp, verified credential scope and full device ID when available, the device name/platform at the time, trusted source IP, allowlisted operation, optional `%pane`, method, status, outcome and duration. Names/platform are descriptive metadata, not identity factors. The master credential is shared and cannot identify the physical device/person using it. A proxy address is trusted only under the existing server proxy rules.

The vocabulary covers authenticated session/terminal/conversation/HQ/knowledge/usage/settings reads, input, focus, creation, upload, sharing, paired-device and notification management, routes and follow-up changes. Unclassified authenticated API requests use “Server request”. Successful code redemption records the resulting verified device/link, never the code. Icons, theme assets, public static pages and health probes are not access events. Existing pairing diagnostics retain rejected redemption reasons separately. Missing/unknown bearer attempts use the existing one-per-minute global sampler, with a summary count in diagnostics; this is not a per-attempt intrusion log.

Successful GET/HEAD reads are recorded once per five-minute window per full identity, operation and pane; they are sampled, not a request count. Read failures and ordinary HTTP mutations are recorded individually. Actual SSE and remote-terminal opens/closes are recorded separately; failed opens remain visible. A force-killed server cannot emit a close receipt. Keystrokes inside an attach stream are not individually recorded.

No authorization value, token, pairing/share code, query string, request/response body, command text, image or conversation text is added. Existing centralized redaction still applies. HTTP acceptance is not proof that an agent finished, and an upload request does not reveal its contents.

## Storage, querying and failures
Reuse `~/.local/share/gtmux/logs/`, with existing `logs.retainDays` and `logs.maxMB` limits (30 days / 100 MiB by default); diagnostics and access records share the same cap. History begins after upgrade. No activity is backfilled or uploaded automatically. Owners can copy/export private records intentionally; diagnostic bundles include them, so review a bundle before sharing it.

The reader returns newest first, 100 records by default, at most 200. Each request scans at most 4 MiB. A page with no matching entries may still offer an earlier cursor. Cursors identify a validated log basename/offset, never arbitrary paths; removed/truncated pages return HTTP 410. Read failures return 503, not empty history. Refresh starts again. Clients keep already-loaded records after pagination errors and ignore late responses from a different server. Phone/iPad virtualize rows and cap readable width; full device IDs are under a disclosure. Native readers use the same page model.

Write failures use the existing bounded stderr warning and doctor write probe. Logging is not a fail-closed gate for terminal input or session management. These logs are useful operational evidence, not a tamper-proof security ledger.

## Local permissions
Diagnostic appends open owned regular files without following the final directory/file symlink, hold the directory descriptor through opening, and narrow existing group/other permissions. gtmux data/config roots and log directory use owner-only access, files at most 0600. Foreign-owned or nonregular files are refused. Read-only history never chmods files and refuses unsafe final links.

Doctor checks the full gtmux data/config trees for POSIX permission exposure, ownership, unreadable paths and symbolic links it cannot verify. `doctor --fix` narrows only owned paths, does not follow links, and leaves unverified cases for manual review. POSIX checks do not certify ACLs, protect against the owner/root, or prevent all privileged filesystem changes.

## Verification
Isolated fixtures cover verified identity/header spoofing, guest/unknown credential denial, read sampling versus individual writes/errors, stream lifecycle and failed opens, secret/payload exclusion, bounded cursor pagination, retention filtering, symlink refusal and permission narrowing. Component tests cover old-server errors, retries, deduplication, server switching and device-ID disclosure. Hosted CI builds the native and iOS clients. Physical-device layout, keyboard and VoiceOver behavior require observed acceptance and are not claimed by unit tests.
