# Tasks — hq-action-journal

Status: IMPLEMENTED (2026-08-16, one pass; every slice's tests landed with it). Perception-layer change: the risky direction is an exclusion
that hides real debt. Every slice pairs its "audit is not debt" test with a
"non-audit control still counts" twin before the mechanism lands.

## 1. Vocabulary + constructors (the substrate every later slice calls)

- [x] 1.1 `internal/events`: add `AuditPrefix = "gtmux:audit:"`, `IsAudit`, and the
      six record constructors (`AuditWakeDelivered`, `AuditWakeDropped`,
      `AuditSend`, `AuditReap`, `AuditRotate`, `AuditHQSession`) in a new
      `audit.go`. Constructors own their bounds: send heads truncate rune-safely,
      every summary is single-line (newlines collapse), and each stamps
      `SevRoutine` explicitly.
- [x] 1.2 Tests first (`audit_test.go`): `IsAudit` ⊂ `IsControl` (an audit record
      is also a control record — the sensor-delta exclusion is inherited, prove
      it); bounds (oversized head truncates at a rune boundary, multibyte safe);
      newline collapse; each constructor's event name and field mapping; a
      round-trip through `Append`/`ReadSince` (HOME-redirected temp dir).

## 2. Audit is trail, not debt (the exclusion set — the risky slice)

- [x] 2.1 `unreadScan` skips `IsAudit` records. Tests BOTH directions: a journal
      whose only post-watermark records are audit yields `N == 0` (and the
      watermark steps over them via the existing pure-echo path); a non-audit
      `gtmux:*` control record past the watermark still counts (the #647
      maintenance-trigger channel must not regress).
- [x] 2.2 `pullView` hides `IsAudit` alongside own-pane and blinks; `--all`
      restores; a non-supervisor read is untouched. Test via the HQ-home cwd seam
      the existing pullView tests use. Update `noteHiddenEcho`'s en+zh strings to
      name the audit trail in the withheld note.
- [x] 2.3 Count/pull consistency: one test asserts the two exclusion sets are the
      same predicate over the same delta (the `hq-unread-noise` invariant, now
      with three exclusion classes).
- [x] 2.4 `countHQTurns` (self-rotate fleet movement) skips `IsControl` records —
      gtmux bookkeeping is never fleet movement. Test: audit + control records
      past the cursor advance neither `Turns` nor `fleet`; a real pane record
      still advances `fleet`. (Without this, every delivered wake's audit record
      re-arms standing knocks — the self-feeding alarm class.)
- [x] 2.5 Playbook: extend the pull-view teaching (hidden set gains "gtmux's own
      audit trail"), bump `hqPlaybookVersion` 22 → 23.

## 3. Wake delivery + drops enter the journal

- [x] 3.1 `hqnudge`: on a confirmed delivery (drainInto `delivered`, and both
      repair-confirmed paths) append `AuditWakeDelivered(id, payload)`. Tests:
      the delivered fake-io path leaves exactly one record carrying the batch id
      and full payload; the sendFailed/unacked/unsubmitted paths leave none.
- [x] 3.2 `hqnudge`: the three drop paths append `AuditWakeDropped` with their
      reason — `evicted` (evictOverflow, payload read before removal),
      `unconfirmed` (requeueUnacked past budget — covers both the drain path and
      the abandoned Enter-repair), `superseded` (claimBatch revalidation drop).
      This implements the control trace `standing-wake-backoff` §2.1 specified.
      Tests per path: the dropped line's text and reason land in the journal; a
      requeue below budget writes nothing; a revalidation RE-RENDER (keep, line
      changed) writes nothing.
- [x] 3.3 Volume guard: a drain that delivers N coalesced entries writes ONE
      wake-delivered record (per batch, not per entry).

## 4. The four spool-only emitters move to the journal

- [x] 4.1 `slowtick.go` wakeWatchdog + feedWatchdog: `hqfeed.EmitControl` →
      `events.Append` (same event names, `SevImportant`). Fix the wake-degraded
      comment that claims pull-side visibility — after this it is finally true.
- [x] 4.2 `hqfeed/daemon.go` cursor-gap `feed-degraded` + startup `reconcile`:
      same move. The daemon tails the journal, so the record spools itself on the
      normal tail — assert ONE spool copy, not two.
- [x] 4.3 Delete `hqfeed.EmitControl` (no callers remain); keep the `Control*`
      constants (the `--tail` renderer still matches them). Tests: each site's
      record reaches the journal via `ReadSince`; these records are NOT audit
      (they count toward unread — the twin direction of 2.1's test).

## 5. The rotation chain

- [x] 5.1 `RotateHQ` appends `AuditRotate(retiring session id, reset input)` after
      the reset is typed. Thin call site over the tested constructor; the settle
      record below is the load-bearing chain.
- [x] 5.2 `selfRotateSensorFor`'s session-change branch appends
      `AuditHQSession(new, old)` before discarding the window — REPLACEMENTS
      only: a first sighting loses nothing (the resume record still names it)
      and the sensors' silence discipline holds (a healthy session writes
      nothing to the stream — the existing test that pins it stays green).
      Tests: first sight (no record), a rotation (both ids), an unchanged tick
      (no record); the state file's overwrite semantics unchanged.

## 6. Send + reap

- [x] 6.1 `cmdSend` text paths (verified, plain-terminal, unverified) append
      `AuditSend(pane, state, head)` at settlement; `--key` sends none. Call
      sites stay thin; the constructor's bounds are already tested in 1.2.
- [x] 6.2 `cmdReap` appends `AuditReap(task id, pane, actions)` when
      `res.Reaped`. A snooze writes nothing (it is already persisted on the task).

## 7. Docs + gates (same PR — the spec ⇄ code ⇄ test rule)

- [x] 7.1 `docs/cli.md`: the events/watermark section documents the audit
      sub-namespace, the widened hidden set, and the now-journal-borne
      degradation records.
- [x] 7.2 `CLAUDE.md`: the hq paragraph's counting-exclusion description gains the
      audit class; the wake-delivery paragraph notes delivery/drop journaling.
- [x] 7.3 Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
      `scripts/check-design.sh` (openspec validate --specs --strict included).
