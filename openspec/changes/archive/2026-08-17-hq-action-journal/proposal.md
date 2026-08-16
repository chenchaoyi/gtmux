# hq-action-journal — the supervisor's own acts enter the journal it keeps about everyone else

Origin: the commander's 2026-08-16 review of HQ's audit trail, prompted by the live
HQ's own ledger entry (seq 15704): "self-audit has been running all along — what's
missing is the exit… a successor lives off a hand-written board; whatever it misses
is gone." Companions: `hq-watermark-wakes` (the consumption guarantee this extends),
`standing-wake-backoff` (whose §2.1 drop trace this finally implements).

Perception-layer change — the wake layer is HQ's hearing; it changes via review.

## Why — every party is journaled except the actor

`events.jsonl` is an append-only, seq-ordered journal of the FLEET's lifecycle, and
the consumption watermark guarantees HQ eventually reads all of it. But the
supervisor's OWN acts leave almost no trace in the stream it reconciles against:

- **A delivered wake records only its batch id.** The hook writes the HQ pane's
  `UserPromptSubmit` with `summary:"#511a19"` — six hex characters. The LINE that
  was typed (which classes, which panes, what figures) exists nowhere after the
  claim files are deleted (`hqnudge.go` drainInto). "What did gtmux tell HQ at
  14:02?" cannot be answered from the journal.
- **A dropped wake records nothing at all.** Three paths remove a queued line with
  a bare `os.Remove`: queue-overflow eviction (`evictOverflow`), the exhausted
  ack budget (`requeueUnacked`), and the delivery-side revalidation drop
  (`claimBatch`). The third is worse than silent: `standing-wake-backoff` §2.1
  SPECIFIED "drop (and record a control trace)" — the trace was never
  implemented, so the spec has promised an audit line the code does not write.
  "Which wakes did HQ never get?" has three permanently-invisible answer classes.
- **`gtmux send` keeps a hash.** The re-send interlock stores `{hash,ts}` per
  pane, overwritten each send. The target pane's hook event carries the prompt's
  normalized head, but nothing marks it as SENT-BY-THE-SUPERVISOR versus typed by
  the user — "what did HQ actually send %28?" reconstructs from neither.
- **A reap erases its own record.** `cmdReap` removes the task file; the journal
  never learns a dispatch was reclaimed, or what the reap did.
- **Rotation overwrites the predecessor.** `selfrotate-state` and the
  `resume/<loc>.json` record keep only the CURRENT session id; the moment the
  sensor adopts the successor, the retiring session's id — the only pointer to
  its transcript — is gone. The measured cost is not hypothetical: the live HQ's
  handoff is a hand-written board, and the commander's question "why did HQ
  decide X last week" is unanswerable the moment the session that decided it
  rotates.
- **Two degradation records are written where no reader looks.** `wakeWatchdog`
  and `feedWatchdog` (and the feed daemon's own cursor-gap and startup-reconcile
  paths) still emit through `hqfeed.EmitControl`, which writes ONLY to the feed
  spool. `#647` (hq-maintenance-triggers) diagnosed exactly this delivery failure
  for distill/self-check and moved them to the journal; these four sites were not
  moved with them. Measured on the live machine (2026-08-16): the spool holds
  50 × `gtmux:feed-degraded`, 16 × `gtmux:wake-degraded`, 9 × `gtmux:reconcile`;
  the journal holds ZERO of any of them. Seventy-five perception-degradation
  records are invisible to every `gtmux events` query — the comment at the
  wake-degraded call site still claims "the pull side sees it", and it is wrong.

The through-line: the journal answers "what happened to the fleet" but not "what
did the supervision DO" — and the second question is the one a successor session,
a self-audit pass, and the commander's "why" all need.

## What changes

**One journal, one new sub-namespace.** Audit records ride the existing
`events.Record` shape and the existing `gtmux:` control namespace, under a
dedicated `gtmux:audit:` prefix:

| Event | Written when | Carries |
|---|---|---|
| `gtmux:audit:wake-delivered` | a wake batch is confirmed landed (drain ack, or the Enter-only repair confirms) | batch id + the full delivered payload (≤ the existing 800-char batch cap) |
| `gtmux:audit:wake-dropped` | any of the three silent-drop paths fires | the drop reason (`evicted` / `unconfirmed` / `superseded`) + the dropped line |
| `gtmux:audit:send` | a `gtmux send` text delivery settles | target pane + outcome state + a bounded payload head |
| `gtmux:audit:reap` | a reap actually reclaims a dispatch | task id + pane + the actions taken |
| `gtmux:audit:rotate` | `gtmux hq --rotate` types the reset | the retiring session id + the agent's reset input |
| `gtmux:audit:hq-session` | the health sensor observes the HQ session id change | successor id + predecessor id — the durable rotation CHAIN |

**Audit records are trail, not debt.** They document acts their actor already
knows about, so they are EXCLUDED from the unread count and from the supervisor's
default pull view (which must show exactly the debt — the `hq-unread-noise`
consistency rule), and `--all` restores them. `IsControl` already covers them (the
prefix nests inside `gtmux:`), so every sensor's zero-change gate excludes them
for free. Without this exclusion the trail would be the loop's fuel: every
delivered wake would mint a new unread record, which would knock, which would
deliver, which would mint — the exact self-feeding alarm `hq-unread-noise` and
`standing-wake-backoff` spent two changes killing. The self-rotate sensor's fleet-
movement counter gets the same guard: control records are gtmux's bookkeeping,
never fleet movement (the events.go doctrine, now enforced at that call site too).

**The four spool-only emitters move to the journal.** `wake-degraded`,
`feed-degraded` (watchdog and daemon cursor-gap), and the daemon's startup
`reconcile` append through `events.Append`, exactly as #647 did for
distill/self-check: the journal record is the audit copy AND the feed daemon
spools it on its normal tail — one record, not two. These are NOT audit records:
they carry new information for HQ, so they count toward unread and reach HQ
through the completeness net even when the wake channel itself is the casualty.
`hqfeed.EmitControl` is deleted once its last caller moves.

**Rotation stops erasing the chain.** The state files keep their runtime scalars
(unchanged); history lives in the journal. `--rotate` records the act with the
retiring id; the sensor's adoption of the successor records the settle with both
ids. "Who preceded this HQ, and where is its transcript" becomes a journal query.

## Non-goals

- **No new wake class, no new knock.** Nothing here types anything into any pane.
- **No change to watermark semantics** — only to which records constitute debt.
- **No knowledge-base provenance** (capture/distill entry-ification) — that is the
  planned follow-up change, which this journal vocabulary is the substrate for.
- **No serve-side `/api/send` audit** — the phone's send path has its own sendID
  idempotency and latency budget; auditing it is a named follow-up, not a silent
  omission.
- **No doctor rows for audit coverage** — follow-up alongside the KB work.
- **The spool stays.** Deleting the now-redundant feed layer is its own decision
  with its own verification (does anything still read `--tail`?); this change
  only stops writing records where no one reads.

## Impact / risk

The risky direction is the exclusion set: hiding too much from the count would
re-open the silent-loss hole the watermark closed. Every slice therefore pairs an
"audit records do not count / do not show" test with a "non-audit control records
still count / still show" twin, and the pull-view/count consistency is asserted in
one shared-predicate test. Volume is bounded by construction: wake batches are
≤ 800 chars, send heads are truncated, and the journal's existing 20 MB × 2
rotation is untouched. The playbook's pull-view description changes (the hidden
set gains "gtmux's own audit trail"), so `hqPlaybookVersion` bumps 22 → 23.
