# Using and reviewing knowledge

[中文](knowledge-review.zh.md)

The append-only ledger remains authoritative. This version adds CLI review and a local Markdown workspace, with resumable reviews through the existing model CLI, without another service, scheduler, database or sync plugin.

## Consult before an action

Run `gtmux knowledge consult --query 'publish while preserving access policy' --action release --repo /path/to/project --budget 1200` before composing the plan or goal. JSON returns full cards, revisions, reasons and verification needs. The estimated budget is 1–2000 tokens, at most six cards; exceptions are never truncated. `missing_required` reports applicable protected cards omitted for review/expiry or budget. Resolve that gap before relying on the result. Retrieval never records a hit or use.

Repeat `--tool name=version` for exact version constraints. `--role hq` selects HQ content, not write authority. Unscoped legacy content is excluded by default; explicit HQ investigation can use `--include-legacy`. Raw search remains available. `--purpose audit|evaluation` labels the purpose. The new HQ seed teaches consult before goal construction. Existing automatic spawn hints are unchanged; worker callers must still consult explicitly. Source edits do not update an active HQ home: adoption requires the normal version/seed upgrade.

## Local Markdown review

```sh
gtmux knowledge review export /path/to/project/kb-vault
gtmux knowledge review diff /path/to/project/kb-vault > /path/to/project/proposal.json
gtmux knowledge review check /path/to/project/proposal.json
```

Choose an empty directory, then open it as an Obsidian vault. Edit titles and bodies; retain the three generated scalar properties. Files may be renamed with their ID intact. File deletion reports a missing copy and never retires knowledge. Personal notes belong in `个人批注/`; new notes without IDs remain drafts for the existing capture/add gate.

Diff produces an inert proposal. The existing HQ/owner writer applies it using `review apply`; do not impersonate HQ by changing directories. A batch is 1–25 changes. Split larger batches, preserving each base revision, and clear proposal `id` to recompute the digest after edits. Conflicts preserve both versions. Notes and the baseline are published atomically; read-only files are refused. Finish editing before export refresh: an external editor does not share the ledger lock. Legacy wiki names may need manual follow-up in Obsidian; exporting does not invent missing knowledge.

Apply preserves identity, recurrence, sources and hypothesis status. A prose edit makes an old action card tentative and drops a stale translation. Distributed content reopens landing; use existing land/sync to verify carriers. Exact historical LOCAL sections can be replaced; manual edits and duplicate markers are conflicts. Apply does not silently update every instruction file.

Sensitive entries and accounts are excluded. If previously exported content later becomes private, refresh reports that the local copy needs explicit removal, including editor history/backups/cloud copies. No cloud sync is enabled. Baselines and properties detect conflicts; they are not authentication or signatures.

## Resumable Dreaming through the existing CLI

On existing distill wakes, HQ checks `dream list` and `dream status <run-id>`, continues unfinished packets and defers new work while proposals await review. There is no additional daemon. Each packet freezes eligible KB IDs/revisions, prose, existing memory metadata, audience and evidence; default 20, maximum 25 items for execution.

The existing HQ writer creates project-owned `run.json`:

```json
{"selection":{"mode":"pilot","limit":20},"max_calls":40,"max_seconds":7200}
```

Run `gtmux knowledge dream start /path/to/project/run.json` and retain its returned `id`. Resume that ID rather than creating another task. Read-only `dream plan <request.json>` still accepts the inner `selection` object for preview.

HQ creates `choice.json`, replacing the model placeholder with an exact locally supported name:

```json
{"agent":"codex","model":"<HQ-selected-model>","effort":"high","reason":"This packet contains disputed scope boundaries","timeout_seconds":600}
```

Both `codex` and `claude` reuse existing CLI authentication. HQ chooses model/effort/reason; later continuations may change models, with every choice retained. Simple editing need not use the strongest model; missing evidence calls for deferral, not invented verification.

```sh
gtmux knowledge dream run <run-id> /path/to/project/choice.json
gtmux knowledge dream status <run-id>
gtmux knowledge dream history <run-id> > /path/to/project/dream-history.json
gtmux knowledge dream proposal <run-id> > /path/to/project/proposal.json
gtmux knowledge review check /path/to/project/proposal.json
```

Check/apply only nonempty proposals. `review_ready` means all items received a disposition, possibly defer; it does not mean knowledge is repaired or changes applied. HQ reviews before the existing `review apply` step. Dreaming never writes the KB, hits or candidate queue itself. Do not impersonate HQ by changing directories.

Each item is a checkpoint: persist its input, choice and timeout before invocation; atomically persist its decision, bounded final response and reported usage afterward. Completed items are skipped. An unfinished item's internal reasoning cannot be resumed; only that item may need another call.

Failures/cancellation/invalid JSON pause without automatic retry. An orphaned start is archived as interrupted/unknown on the next run, which stops without relaunching. HQ verifies that the prior process has ended before another deliberate run. Any unresolved attempt also blocks other runs. A global nonblocking process lock excludes simultaneous runners. Changed, retired or newly private inputs are deferred before invocation.

After at most three attempts per item, inspect or use `dream defer <run-id> <reason.json>`, with `{"item_id":"topic/id","reason":"Current evidence needs separate investigation"}`. Deferral keeps earlier charges and history, permits remaining work and does not mark the version reviewed.

After an explicit owner allowance decision, HQ may run `gtmux knowledge dream extend <run-id> <allowance.json>`, for example `{"max_calls":6,"max_seconds":1800,"reason":"Owner authorized completion of this acceptance"}`. Values are absolute total ceilings, can only increase, and remain within 60 calls/4 hours per run. The reason and ceilings append to the original journal; an identical retry adds nothing. The fixed plan, prior costs, unknown usage, three-attempt item guard and shared weekly limits remain. Extension launches no model. Recover unresolved attempts first; completed packets need no increase.


### Operational and token limits

| Constraint | Execution behavior |
|---|---|
| Calls per packet | Default 40; configurable 1–60, including failures/interruption |
| Time per packet | Default 2 hours; configurable 1 second–4 hours |
| Per-call timeout | Required 1–1800 seconds, shortened to remaining packet/week time |
| All runs together | 200 calls and 12 execution hours per UTC Monday-based week |
| Retry | Maximum three attempts per item; no automatic failure retry |
| Tokens | Report terminal usage; stop before another call when a threshold is reached |

Input/generation thresholds: pilot 500k/200k and bootstrap 6M/1.8M accumulate across their packets over the full retained history; routine counts all modes in the current week against 1M/400k. New weeks do not reset pilot/bootstrap. Read-only plan still supports advisory `allow_burst`; CLI execution explicitly rejects that override.

**The CLI cannot guarantee an in-flight token or billing cap.** Missing receipts remain unknown, never zero. Use reported generation, not visible-text estimates of hidden reasoning. Advisory `usage_known:false/can_start:false` still means token affordability is unverified; the CLI route executes under separate operational bounds. No new API credentials or billing service are introduced. Timeout kills the controller's child process group, but cannot prove immediate remote billing cancellation. After abrupt controller death an unconfirmed child may remain: HQ inspection is required, and remote exactly-once billing is not promised. Unsupported isolation flags/protocols fail instead of silently loosening access.

Failure history retains a fixed-vocabulary `failure_code` and numeric `exit_code` for configuration, authentication, network, quota, cancellation, protocol and process failures. Categories are diagnostic hints, not proof of root cause. At most the last 8 KiB of stderr stays in memory for classification; raw diagnostics are never archived. Failed calls and unknown usage remain charged to the original run when resumed after a fix.

### Archive and recovery

Managed state is `knowledge-dream/actions.jsonl` under gtmux's state directory: mode 0600, sequential hash chain, atomic fsync/rename/directory-sync publication. It is the sole execution state. List/status/history/proposal are read-only. Archive only bounded final assistant prose and normalized decisions, not hidden reasoning, tool streams or stderr. The chain detects accidental corruption; it is not an authenticity signature.

Stop at 16 MiB or corruption/incomplete records and preserve the file. This version has no journal compaction or segment import. Do not remove/truncate it to make room: that would discard progress and budget totals. Recovery requires a stopped executor, retained original backup and a validated complete chain. Keep the whole journal across rollback; pause Dreaming while an older binary lacks these commands.

Calls use a disposable empty directory with tools/plugins/session persistence disabled. Sensitive/accounts entries are excluded; revisions are checked before and after calls. No native-session collection occurs. Evidence references must be in the packet or name the frozen KB revision. Model proposals cannot newly verify facts or widen established scope.

## Usage and recovery

`knowledge usage record <receipt.json>` records explicit self-reports separately from hits. `usage summary` covers 30 days. Receipts carry version 1, hashed task/plan keys, purpose, stage, observation `self_report`, and item IDs/revisions/card hashes. Stages: retrieved, shown, adopted, outcome; outcomes: succeeded, failed, unknown. Duplicate receipts are idempotent. Evaluation is separate, audits are rejected, and storage stops at 8 MiB pending explicit archival.

Missing reports do not mean unused, and reported success does not prove causal improvement. No trusted receiver receipt or A/B result is claimed. History remains intact. Restore erroneous prose with a new proposal against the current revision, not by overwriting a whole ledger containing concurrent work. Retired content uses existing history/add recovery. Back up ledger and workspace before rollout. Older schema-3 readers retain basic fields but do not understand new memory/usage semantics; pause these new writes during a downgrade.
## Landing a reviewed revision

Automatic `gtmux knowledge land <id>` checks the version, writes its carrier and
records completion under the existing ledger lock. Same-ID review reopens landing;
a delayed old carry cannot settle or overwrite the new revision.

For a lesson you carried yourself, first read `gtmux knowledge show <id> --json`,
verify that exact text reached the destination, then use
`gtmux knowledge land <id> --ref '<destination>' --revision '<revision from that read>'`.
A missing or stale revision is refused. Read again and check the actual destination
before confirming again; do not simply substitute a fresh token. Mac and mobile
confirmation keep the version originally viewed. Older clients without a revision
must refresh/upgrade for manual landing; automatic carry remains available.

LOCAL updates preserve unrelated manual rules and atomically replace complete private
bytes (0600). Read-only/link targets and edited generated sections require review.
If publication fails, landing stays pending. If bytes reached disk before a later
error, retry is idempotent; inspect the receipt before deciding whether to retry or
repair rendered views. Historical landing records remain readable without migration.
