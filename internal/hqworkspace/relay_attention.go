package hqworkspace

import (
	"slices"
	"strings"
)

type RelayAttentionDue struct {
	Reconciliation
	SourceEnded bool
}

// RelayAttentionQueue sorts by actual source end evidence, then by the
// reconciliation queue's stable intake order. Matter assignment is optional.
func RelayAttentionQueue(d Database, now int64) ([]RelayAttentionDue, bool) {
	var paired, unpaired []RelayAttentionDue
	enrolled := false
	ends := map[Conversation][]int64{}
	for _, q := range d.Reconciliations {
		o := q.Observation
		enrolled = enrolled || o.RelayAttention
		switch o.Kind {
		case "Stop", "StopFailure", "triage-unassigned:Stop", "triage-unassigned:StopFailure",
			"triage-ambiguous:Stop", "triage-ambiguous:StopFailure":
			if o.OccurredAt <= now {
				source := relayAttentionSource(o.Conversation)
				ends[source] = append(ends[source], o.OccurredAt)
			}
		}
	}
	for _, observed := range ends {
		slices.Sort(observed)
	}
	for _, q := range ReconciliationQueue(d, now) {
		if !relayAttentionOwed(q, now) {
			continue
		}
		o := q.Observation
		observed := ends[relayAttentionSource(o.Conversation)]
		first, _ := slices.BinarySearch(observed, o.OccurredAt)
		row := RelayAttentionDue{Reconciliation: q, SourceEnded: first < len(observed) && observed[first]-o.OccurredAt <= 5*60}
		if row.SourceEnded {
			paired = append(paired, row)
		} else {
			unpaired = append(unpaired, row)
		}
	}
	return append(paired, unpaired...), enrolled
}

func relayAttentionSource(c Conversation) Conversation {
	return Conversation{Agent: c.Agent, HomeID: c.HomeID, SessionID: c.SessionID}
}

func relayAttentionOwed(q Reconciliation, now int64) bool {
	o := q.Observation
	if !o.RelayAttention || o.Kind != "relay-report" || !strings.HasPrefix(o.Ref, "relay:") ||
		o.OccurredAt <= 0 || o.OccurredAt > now || hasLiveClaim(q, now) {
		return false
	}
	return q.State == "pending" || q.State == "leased" || relayAttentionRecheckDue(q, now)
}

func relayAttentionRecheckDue(q Reconciliation, now int64) bool {
	return q.State == "deferred" && q.NextCheck > 0 && q.NextCheck <= now
}

// ReserveRelayAttention consumes a scheduling opportunity, never a source
// claim. It is intentionally independent of delivery/assessment state.
func (s *Store) ReserveRelayAttention(ref string, now int64) (reserved bool, err error) {
	err = s.transaction(func(d *Database) error {
		q := d.Reconciliations["relay-"+ref]
		if !relayAttentionOwed(q, now) || q.Observation.Ref != "relay:"+ref || q.RelayAttentionAt != 0 {
			return nil
		}
		q.RelayAttentionAt = now
		d.Reconciliations[q.Observation.ID] = q
		changed(d, now, "relay-attention-reserved", q.Observation.MatterID, q.Observation.ID, "protected scheduling opportunity")
		reserved = true
		return nil
	})
	return
}

func RelayAttentionPending(d Database, ref string, now int64) (Reconciliation, bool) {
	q := d.Reconciliations["relay-"+ref]
	return q, q.Observation.Ref == "relay:"+ref && relayAttentionOwed(q, now)
}
