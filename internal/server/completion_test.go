package server

import (
	"testing"
	"time"
)

func TestCompletionAlertsRequireEvidence(t *testing.T) {
	current := AgentStatus{PaneID: "%711", Agent: "Claude Code", Status: "working"}
	var alerts []Alert
	h := newHub(func() []AgentStatus { return []AgentStatus{current} }, time.Hour, func(a Alert) { alerts = append(alerts, a) })
	h.tick()
	current.Status = "idle" // only the screen became quiet; no lifecycle event
	h.tick()
	if len(alerts) != 0 {
		t.Fatalf("screen-only idle must not notify: %+v", alerts)
	}
}

func TestCompletionPendingBoundaries(t *testing.T) {
	working := AgentStatus{PaneID: "%1", Agent: "Codex", Instance: "session-one", Status: "working", Since: 100}
	idle := working
	idle.Status, idle.Confirmed, idle.CompletionID, idle.CompletedAt = "idle", true, "turn-one", 101*int64(time.Second)
	unconfirmed := idle
	unconfirmed.Confirmed, unconfirmed.CompletionID, unconfirmed.CompletedAt = false, "", 0
	for _, tc := range []struct {
		name   string
		states []AgentStatus
		want   int
	}{
		{"startup does not replay", []AgentStatus{idle, idle}, 0},
		{"normal completion", []AgentStatus{working, idle, idle}, 1},
		{"repaint does not replay completion", []AgentStatus{idle, working, idle}, 0},
		{"delayed proof", []AgentStatus{working, unconfirmed, unconfirmed, idle, idle}, 1},
		{"disappearance cancels pending", []AgentStatus{working, {}, idle}, 0},
		{"instance change cancels pending", []AgentStatus{working, func() AgentStatus { a := idle; a.Instance = "replacement"; return a }()}, 0},
		{"failure cancels pending", []AgentStatus{working, func() AgentStatus { a := unconfirmed; a.EndReason = "error"; return a }(), idle}, 0},
		{"unknown state cancels pending", []AgentStatus{working, func() AgentStatus { a := unconfirmed; a.Status = "running"; return a }(), idle}, 0},
		{"old proof cannot finish newer work", []AgentStatus{working, func() AgentStatus { a := idle; a.CompletedAt = 99 * int64(time.Second); return a }()}, 0},
		{"supervisor never notifies", []AgentStatus{func() AgentStatus { a := working; a.Role = "supervisor"; return a }(), func() AgentStatus { a := idle; a.Role = "supervisor"; return a }()}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var current []AgentStatus
			var alerts []Alert
			h := newHub(func() []AgentStatus { return current }, time.Hour, func(a Alert) { alerts = append(alerts, a) })
			for _, s := range tc.states {
				current = nil
				if s.PaneID != "" {
					current = []AgentStatus{s}
				}
				h.tick()
			}
			if len(alerts) != tc.want {
				t.Fatalf("alerts=%+v, want %d", alerts, tc.want)
			}
		})
	}
}

func TestCompletionPendingRespectsConsent(t *testing.T) {
	a := AgentStatus{Agent: "Codex", SessionID: "one", HomeID: "home", Client: "unconfirmed", Status: "working", Notify: true}
	var alerts []Alert
	h := newHub(func() []AgentStatus { return []AgentStatus{a} }, time.Hour, func(a Alert) { alerts = append(alerts, a) })
	h.tick()
	a.Status, a.Notify = "idle", false
	h.tick()
	a.Notify, a.Confirmed, a.CompletionID = true, true, "one"
	h.tick()
	if len(alerts) != 0 {
		t.Fatalf("re-enrollment replayed a pending completion: %+v", alerts)
	}
}
