package server

import (
	"testing"
	"time"
)

// Lifecycle evidence may be observed one poll after the screen becomes quiet.
func TestDelayedCompletionEvidenceEmitsOneDone(t *testing.T) {
	current := AgentStatus{PaneID: "%711", Agent: "Claude Code", Status: "working"}
	var alerts []Alert
	h := newHub(func() []AgentStatus { return []AgentStatus{current} }, time.Hour, func(a Alert) { alerts = append(alerts, a) })
	h.tick()
	current.Status = "idle"
	h.tick()
	if len(alerts) != 0 {
		t.Fatal("unconfirmed idle must not notify")
	}
	current.Confirmed = true
	current.CompletionID = "one"
	h.tick()
	h.tick()
	if len(alerts) != 1 || alerts[0].Kind != "done" {
		t.Fatalf("expected one completion after delayed lifecycle evidence, got %+v", alerts)
	}
}
