package hqworkspace

import "testing"

func TestHRF002DueDeferredReportKeepsItsConsumedReservation(t *testing.T) {
	d := Empty()
	entry := Reconciliation{State: "deferred", NextCheck: 1400, RelayAttentionAt: 1100, Observation: Observation{
		ID: "relay-report-old", Kind: "relay-report", Ref: "relay:report-old", RelayAttention: true,
		Continuation: true, OccurredAt: 1000,
	}}
	d.Reconciliations[entry.Observation.ID] = entry
	if got := ReconciliationWakeQueue(d, 1399); len(got) != 0 {
		t.Fatal("scheduled an early recheck", got)
	}
	if got := ReconciliationWakeQueue(d, 1400); len(got) != 1 || got[0].RelayAttentionAt != 1100 {
		t.Fatal("due deferred report lost its recheck or its consumed reservation", got)
	}
}

func TestHRF003PairedRelayRetainsEarlierMatchingStop(t *testing.T) {
	d := Empty()
	c := Conversation{Agent: "codex", HomeID: "one-home", SessionID: "one-thread"}
	report := func(key string, when int64) Reconciliation {
		return Reconciliation{State: "pending", Observation: Observation{ID: "relay-" + key, Ref: "relay:" + key,
			Kind: "relay-report", RelayAttention: true, OccurredAt: when, Conversation: c}}
	}
	d.Reconciliations["relay-first"] = report("first", 1000)
	d.Reconciliations["relay-earlier"] = report("earlier", 500)
	end := func(id, kind string, when int64, source Conversation) {
		d.Reconciliations[id] = Reconciliation{Observation: Observation{ID: id, Kind: kind, OccurredAt: when, Conversation: source}}
	}
	otherHome := c
	otherHome.HomeID = "another-home"
	end("event-false", "Stop", 510, otherHome)
	end("event-close", "triage-ambiguous:Stop", 1100, c)
	end("event-late", "Stop", 1600, c)
	rows, _ := RelayAttentionQueue(d, 1700)
	if len(rows) != 2 || rows[0].Observation.ID != "relay-first" || !rows[0].SourceEnded || rows[1].SourceEnded {
		t.Fatalf("wrong source-end ranking: %+v", rows)
	}
}
