package hqworkspace

// A wake is a request for HQ to claim evidence. The lease, rather than the
// enqueue or transport receipt, shows that a verified HQ started handling it.
const (
	continuationRetry     = 30
	continuationBackoff   = 300
	continuationFastTries = 3
)

func continuationNeeded(d Database, m Matter, o Observation, now int64) bool {
	if m.ID == "" || o.OccurredAt > now {
		return false
	}
	q := d.Reconciliations[goalKey(m)]
	a := d.Assessments[q.ReceiptID]
	if a.Assessment.Decision == "deferred" && o.OccurredAt < a.At {
		return false
	}
	g := GoalResponsibility(d, m, now)
	if goalOwed(g) {
		return true
	}
	// This target stopped after reporting in-turn work. Ask HQ to check it
	// before the older drive deadline, leaving the original action fenced.
	if g.State != "driving" || (o.Kind != "Stop" && o.Kind != "StopFailure") {
		return false
	}
	p := m.Responsibility
	proof := d.Reconciliations[g.ObservationID].Observation
	return p != nil && p.Intent != nil && p.Intent.Target.Same(o.Conversation) &&
		proof.DriveProof != nil && (proof.DriveProof.State == "received" || proof.DriveProof.State == "progress") &&
		o.OccurredAt >= proof.OccurredAt
}

func reopenDeferredGoal(d *Database, o Observation, now int64) {
	m := d.Matters[CanonicalMatter(*d, o.MatterID)]
	key := goalKey(m)
	q := d.Reconciliations[key]
	if q.State != "deferred" || !continuationNeeded(*d, m, o, now) {
		return
	}
	q.State = "pending"
	q.NextCheck = 0
	q.Generation++
	d.Reconciliations[key] = q
	changed(d, now, "goal-recheck", m.ID, key, o.ID)
}

// Claim coverage belongs to this record alone. Neither a sibling claim nor
// a matching wall-clock second says anything about whether HQ saw this evidence.
func hasLiveClaim(q Reconciliation, now int64) bool {
	return q.State == "leased" && q.LeaseUntil > now
}

// ContinuationWakeQueue lists exact participant evidence that still needs an
// HQ turn. It is also evaluated at drain time, after a draft or rotation clears.
func ContinuationWakeQueue(d Database, now int64) []Reconciliation {
	var out []Reconciliation
	for _, q := range ReconciliationQueue(d, now) {
		o := q.Observation
		if !o.Continuation || hasLiveClaim(q, now) {
			continue
		}
		matter := CanonicalMatter(d, o.MatterID)
		if continuationNeeded(d, d.Matters[matter], o, now) {
			out = append(out, q)
		}
	}
	return out
}

// ReconciliationWakeQueue is the existing periodic wake's work. Live claims
// stay visible in the CLI ledger, while the background wake skips them.
func ReconciliationWakeQueue(d Database, now int64) []Reconciliation {
	var out []Reconciliation
	for _, q := range ReconciliationQueue(d, now) {
		if q.Observation.Continuation || hasLiveClaim(q, now) {
			continue
		}
		out = append(out, q)
	}
	return out
}

func dueContinuations(d Database, rows []Reconciliation, now int64) []Reconciliation {
	lastAttempt := map[string]int64{}
	for _, q := range rows {
		id := CanonicalMatter(d, q.Observation.MatterID)
		lastAttempt[id] = max(lastAttempt[id], q.ReminderAt)
	}
	var due []Reconciliation
	for _, q := range rows {
		id := CanonicalMatter(d, q.Observation.MatterID)
		gap := int64(continuationRetry)
		if q.ReminderCount >= continuationFastTries {
			gap = continuationBackoff
		}
		if (q.ReminderAt == 0 || now >= q.ReminderAt+gap) &&
			(lastAttempt[id] == 0 || now >= lastAttempt[id]+continuationRetry) {
			due = append(due, q)
		}
	}
	return due
}

// RemindContinuations records enqueue attempts so polling/restarts coalesce
// them. An attempt neither clears the source evidence nor extends an HQ lease.
func (s *Store) RemindContinuations(snapshot Database, now int64) (rows []Reconciliation, err error) {
	if len(dueContinuations(snapshot, ContinuationWakeQueue(snapshot, now), now)) == 0 {
		return nil, nil
	}
	err = s.transaction(func(d *Database) error {
		rows = ContinuationWakeQueue(*d, now)
		due := dueContinuations(*d, rows, now)
		for _, q := range due {
			q.ReminderAt = now
			q.ReminderCount++
			id, matter := q.Observation.ID, q.Observation.MatterID
			d.Reconciliations[id] = q
			detail := "awaiting HQ claim"
			if q.ReminderCount >= continuationFastTries {
				detail = "HQ has not claimed after fast wake retries; check wake delivery and HQ identity"
			}
			changed(d, now, "continuation-reminder", matter, id, detail)
		}
		if len(due) == 0 {
			rows = nil
		}
		return nil
	})
	return
}
