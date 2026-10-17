# Server access history

## Purpose and entry points
An owner can review which paired device or browser accessed this server, when, which operation was requested and its HTTP result. This is a request audit, not a conversation archive or an agent-completion log.

- Terminal: `gtmux logs --access`; add `--json` for a paginated object, `--before <cursor>` for an earlier page, `--limit 1..200`, `--since 3d`, or `--actor <full-device-id>`.
- Menu bar: Preferences → paired devices → Access history. The local window reads the CLI projection.
- Phone and iPad: Settings → Manage server → Access history. The shared screen has refresh, earlier pages and explicit unsupported-server/permission/expired-page states. Guests cannot open this view or query its API.
- Web: requests are audited; an owner API is available. A browser history viewer is not included in this change.

## Record and privacy boundaries
The diagnostic store holds informational `serve/access.request` action records. Each contains an operation ID, timestamp, verified credential scope and full device ID when available, the device name/platform at the time, trusted source IP, allowlisted operation, optional `%pane` or a structured Codex session UUID, method, status, outcome and duration. Names/platform are descriptive metadata, not identity factors. The master credential is shared and cannot identify the physical device/person using it. A proxy address is trusted only under the existing server proxy rules.

The vocabulary covers authenticated session/terminal/conversation/HQ/knowledge/usage/settings reads, input, focus, creation, upload, sharing, paired-device and notification management, routes and follow-up changes. The route-derived coverage test requires every registered authenticated API to have an explicit category (or an intentional asset/stream exception); unknown requests still use “Server request”. Notification-registration reads have their own category. Successful code redemption records the resulting verified device/link, never the code. Icons, theme assets, public static pages and health probes are not access events. Existing pairing diagnostics retain rejected redemption reasons separately. Missing/unknown bearer attempts use the existing one-per-minute global sampler, with a summary count in diagnostics; this is not a per-attempt intrusion log.

Successful GET/HEAD reads are recorded once per five-minute window per full identity, operation and pane/session; they are sampled, not a request count. Read failures, ordinary HTTP mutations and every HQ archive download are recorded individually. New-session receipts include the returned pane. Desktop transcript/follow-up requests retain only a UUID locator after validating the server home, never an arbitrary query value. An archive generation error is marked failed even if its HTTP 200 headers were already sent; failure before output returns HTTP 500, and unsupported archive methods return 405. Actual SSE and remote-terminal opens/closes are recorded separately; failed opens remain visible. A force-killed server cannot emit a close receipt. Keystrokes inside an attach stream are not individually recorded.

No authorization value, token, pairing/share code, query string, request/response body, command text, image or conversation text is added. Existing centralized redaction still applies. HTTP acceptance is not proof that an agent finished, and an upload request does not reveal its contents.

## Storage, querying and failures
Reuse `~/.local/share/gtmux/logs/`, with existing `logs.retainDays` and `logs.maxMB` limits (30 days / 100 MiB by default); diagnostics and access records share the same cap. History begins after upgrade. No activity is backfilled or uploaded automatically. Owners can copy/export private records intentionally; diagnostic bundles include them, so review a bundle before sharing it.

The reader returns newest first, 100 records by default, at most 200. Each request scans at most 4 MiB. A page with no matching entries may still offer an earlier cursor. Cursors identify a validated log basename/offset, never arbitrary paths; removed/truncated pages return HTTP 410. Read failures return 503, not empty history. Refresh starts again. Clients keep already-loaded records after pagination errors and ignore late responses from a different server. Phone/iPad virtualize rows and cap readable width; native and mobile use compact date-grouped log rows rather than cards. Local 24-hour clocks include seconds; day headings follow the selected English/Chinese UI language, independently of the system locale. Device, operation/target and HTTP outcome are visible with inline platform/IP/full device ID/method/duration metadata. No disclosure is needed. Text remains selectable and native rows offer Copy record. Native readers use the same page model.

Write failures use the existing bounded stderr warning and doctor write probe. Logging is not a fail-closed gate for terminal input or session management. These logs are useful operational evidence, not a tamper-proof security ledger.

## Local permissions
Diagnostic appends open owned regular files without following the final directory/file symlink, hold the directory descriptor through opening, and narrow existing group/other permissions. gtmux data/config roots and log directory use owner-only access, files at most 0600. Foreign-owned or nonregular files are refused. Read-only history never chmods files and refuses unsafe final links.

Doctor checks the full gtmux data/config trees for POSIX permission exposure, ownership, unreadable paths and symbolic links it cannot verify. `doctor --fix` narrows only owned paths, does not follow links, and leaves unverified cases for manual review. POSIX checks do not certify ACLs, protect against the owner/root, or prevent all privileged filesystem changes.

## Verification
Isolated fixtures cover verified identity/header spoofing, guest/unknown credential denial, read sampling versus individual writes/errors, stream lifecycle and failed opens, secret/payload exclusion, bounded cursor pagination, retention filtering, symlink refusal and permission narrowing. Component/presentation tests cover old-server errors, retries, deduplication, server switching, inline full identities, language selection and local midnight/date boundaries. Native image tests guard density and actual metadata drawing. Audit regressions cover route inventory, repeated exports and generation failure, separate desktop targets, returned creation panes and UUID-only storage. Hosted CI builds the native and iOS clients. Physical-device layout, keyboard and VoiceOver behavior require observed acceptance and are not claimed by unit tests.

## Coverage review

| Operation family | Recording policy | Target / boundary |
| --- | --- | --- |
| Sessions, terminal, conversations, HQ, knowledge, usage and settings reads | Successful reads sampled every five minutes; failures individual | Verified identity + operation + pane or desktop UUID; notification-registration reads explicitly classified |
| Input, focus, creation, uploads, sharing, device/notification management, routes, server mode, follow-up and knowledge changes | Every request, including refused/failed attempts | Pane when supplied; creation uses returned pane, desktop follow-up uses structured UUID |
| HQ archive downloads | Every download, without read sampling | Failed generation remains failed, including after committed HTTP 200 |
| SSE / remote terminal | Actual open/close and failed open | No per-keystroke contents; abrupt process death can prevent a close record |
| Assets, static pages and health probes | Excluded | No authenticated operation to attribute; unknown bearer rejection remains globally sampled |

The follow-up corrected export sampling, desktop target collisions, missing creation panes, unclassified push-registration reads, and false-success archive failures. Existing records are unchanged; omitted past activity cannot be reconstructed. Local CLI actions that do not access the HTTP server remain in the existing diagnostic/action journal rather than this external-access projection.

## Filtering and device summaries
The native window and phone/iPad offer Logs and Devices, exact device selection or full-ID entry, operation and outcome filters, and Clear. Filters run in the core reader before paging, not on only visible rows. The operation menu consumes the shared safe vocabulary. Selecting a device summary opens its log and preserves other filters. Different pairing IDs never merge because names match; renamed IDs keep the latest recorded metadata. IDs represent pairings, not hardware; records without IDs stay actor groups, not guessed physical devices.

Summaries show recorded-activity/refusal/failure counts and latest local time. They are incremental under the 4 MiB scan budget and distinct-group page limit. While a continuation exists, the UI says Partial summary and offers Continue summary. Counts are not unsampled request volume. Refresh replaces totals; pagination accumulates by full key without overwriting newer metadata. Switching filters starts over and invalidates late replies; switching mobile servers also clears discovered identities and filters. Old servers that ignore query parameters show an update requirement. No auto-scan or additional database is introduced.
