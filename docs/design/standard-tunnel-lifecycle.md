# Standard tunnel lifecycle and recovery

[中文设计](standard-tunnel-lifecycle.zh.md)

Status: implementation and regression verification; production rollout pending. This design supersedes the Standard reaper and
replacement-address policy in `remote-access-tunnel.md`.

## User contract

A Mac that has opted into always-on Standard access keeps its address and existing
phone/iPad pairings across sleep, network outages, long absences and reclaimed or
accidentally deleted Cloudflare tunnel instances. Returning after 90 days restores
connectivity automatically once the Mac has network access. Previously affected
installations with the old hostname and device ID are included; users should not
need to re-enable access or scan a new QR code.

The address belongs to the device identity, not to a Cloudflare tunnel instance.
Local serve credentials and enrolled-device permissions are separate again. Recovery
never creates, rotates, or restores revoked phone credentials. Off, Local network
and Direct remain deliberate choices: automatic recovery cannot turn Standard back on.
Deleting local identity/credentials is an explicit reset and is not auto-recoverable.
An already-installed old binary needs an update to gain the recovery watcher.

## Failure analysis

The previous reaper treated `conns_active_at == null` as "never connected". Cloudflare
uses that value for a currently inactive tunnel, including previously connected ones.
It also compared an online tunnel's connection-start timestamp against 90 days.
Consequently a recently disconnected established tunnel could take the 24-hour junk
path, and a continuously online tunnel could take the 90-day path. Clearing its
connections before deletion defeated Cloudflare's protection against deleting live
tunnels. Deleting DNS first could break a tunnel even when tunnel deletion failed.

An observed deletion aligned with the deployed daily schedule and this predicate.
The predicate defect is confirmed; unavailable audit permissions mean attribution
of the particular deletion to the scheduled actor is not independently confirmed.
Private incident identifiers and credentials are deliberately outside this document.

Cloudflare field definitions: [tunnel details](https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/subresources/cloudflared/methods/get/).

## Reclaim policy

| Fresh provider state | Decision |
| --- | --- |
| healthy/degraded, any active timestamp or live connection | Keep, regardless of age |
| down, no active connection, valid `conns_inactive_at` | Reclaim only after 90 complete days since disconnection |
| explicitly inactive, no connection timestamps | Reclaim only after 24 complete hours since creation |
| missing fields, unknown status, conflicting/malformed/future timestamps, API error | Keep |
| confirmed deleted/404 | Already absent; keep the device/address reservation |

Only registered device-owned instances are eligible. A `gtmux-` prefix alone is not
proof of ownership. Fetch current details inside the device coordinator immediately
before deletion. Never drain/delete connections to force deletion. Delete the tunnel
first; then delete only the DNS CNAME whose exact target is the deleted instance.
Unfinished operations are not exempt: resolve a pending instance by its persisted ID
or exact opaque operation name, then apply the same fresh-state policy. Reclaiming
it resets the operation journal for a future retry and retains the old identity/route.
A failure retains the registry and is retryable. The hostname is never reassigned to
another device, even when its DNS record is absent. No claim of atomicity with external
Cloudflare connector registration: Cloudflare must reject deletion of live tunnels.

## Coordination and persistent state

A SQLite Durable Object per device serializes provision, repair, replacement and
reclaim, including across Worker isolates. Network awaits do not hold a global lock
or `blockConcurrencyWhile`. A request queue is local to this one object; persistent
state is written before external side effects. KV remains a discovery/index shadow
and legacy import source, not the authority after migration. Import happens once
per object, retaining the exact old label and hostname. KV failure must not overwrite
newer object state with a stale legacy record. Shadow writes are coalesced to respect
KV’s one-write-per-second/key limit; a durable alarm retries interrupted writes.

A device record contains stable `label`/`hostname`, the current `tunnelId`, and, when
needed, a pending operation with an opaque unique provider name and a created ID.
Persist the operation name before POST/create; after an uncertain result, search for
that exact name before creating again. A provider name conflict retains the same
operation for lookup/retry; it must not allocate a different name to bypass the conflict
([Cloudflare client conflict handling](https://github.com/cloudflare/cloudflared/blob/master/cfapi/tunnel.go)).
Multiple matches or unrelated DNS targets
are conflicts, never permission to delete or overwrite another resource.

Replacement is allowed only after a successful provider response containing a
nonempty deletion timestamp, or HTTP 404. Timeouts, TLS errors, 401/403, 429, 5xx,
malformed responses and stale connector tokens are not proof of deletion. Retries
reuse the pending ID. Configure ingress and retrieve a usable connector token before
switching DNS. Persist completion only after the route is ready. DNS may point to
only the known current or pending instance; foreign CNAMEs fail closed.

The old device KV record is sufficient to recover an already misreaped instance:
import that identity, confirm its old UUID is absent, create a new instance, route the
original hostname to it, and return a new connector token. No new public label.

Creation caps remain a soft KV gate, not authentication. First-creation requests
consume a per-IP/global allowance; recovery of an existing device skips the first-enrollment
per-IP cap (so multiple affected Macs behind one NAT can return together), while still
respecting global capacity. Retries of an already journaled instance do not
consume it again. Scheduled traversal is paginated and bounded (100 discovery keys per daily tick);
the cursor continues the next day. Reclamation decrements the approximate active count
once using a durable marker. A separate read-only account scan (at most ten pages)
reconciles quota drift; incomplete/error scans never replace the count with zero or
a smaller unproven total. Failed soft-counter writes do not strand journaled resources. Unattributed historical orphans
remain for a separately reviewed operator cleanup, not speculative deletion.

## Mac recovery and user intent

The always-on serve process uses cloudflared registered connections (`/ready`), with
legacy metrics fallback. After a connection grace interval, a background single-flight
watcher asks the provisioner to recover. Work has a deadline and bounded exponential
backoff; normal cloudflared reconnect remains responsible for transient networking.
A provider response returning the same connector token requires no service reload.

Before requesting, snapshot the Standard agent, serve agent, device ID and URL. At
application time acquire the same short cross-process lock used by remote-access
mode changes and require all snapshot files to match. If Off/Local/Direct, explicit
reinstall, identity reset or credential changes happened in flight, discard the
response. Never create a missing identity, missing serve agent or missing credentials.

Use the new `/standard/recover` endpoint for automatic recovery. A legacy Worker
returns 404 without running its old address-rotating `/provision` code. Primary and
fallback share a bounded request deadline. Require an acknowledged recovery response
with the exact saved HTTPS address.
An old control plane that changes the address cannot silently trigger re-pairing.
Update only the cloudflared agent/token, retaining its binary, arguments, local port,
serve process, URL and phone credential stores. Write atomically with mode 0600.
Persist a hash-only local application marker before changing the agent; retain it
until load succeeds so an interrupted/failed reload is retried even when the saved
token is already updated. Reload cloudflared only; do not restart serve, print tokens/QR codes, or change network,
Clash, VPN, DNS resolver or unrelated services. Failed local application is reported
and retried; a successful provision is not a claim that the edge connection works.

## Verification and rollout

Regression coverage must exercise provider-realistic null fields, a brief offline
established tunnel, an online tunnel older than 90 days, exact threshold boundaries,
unknown state, confirmed deletion, uncertainty, DNS conflicts, partial failure and
retry, concurrent requests, reclaim/recovery serialization, durable-state restart,
legacy import, previously deleted device recovery, unchanged pairing credentials,
and user Off/Local/Direct while an automatic request is in flight.

Build/typecheck the Worker and CLI, run both existing suites and the added runtime
coordination tests, and dry-run deployment including the new DO migration. Deploy
the fixed control plane before shipping the watcher; old CLI recovery remains
compatible, but can now retain the old hostname. Verify one formerly deleted device
with the original URL and existing pairing, then monitor actual registered connections.
Do not describe a build or dry-run as a production deployment.

Rollback must retain the DO binding/class and persistent journal. Reverting to the
old KV-only writer/reaper would lose coordination and can repeat the incident.
Disable reclaim first if production validation fails; do not restore the old predicate
or force-delete connections. This design does not add a hostname-release policy.
