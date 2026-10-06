package server

import (
	"testing"
	"time"
)

// Native sessions have no pane; the hub told them apart by PaneID alone, so every one was
// "" and the last overwrote the others. Two unchanged sessions, idle and working, then
// raised a false "done" on the second tick (%12, 2026-10-06). Each is its own row, by its
// session id, and only a real transition alerts.
func TestHubKeepsNativeSessionsApart(t *testing.T) {
	var alerts []Alert
	var cur []AgentStatus
	h := newHub(func() []AgentStatus { return cur }, 0, func(a Alert) { alerts = append(alerts, a) })
	h.renudge = time.Hour

	a := AgentStatus{Agent: "Claude Code", Status: "idle", SessionID: "native-a"}
	b := AgentStatus{Agent: "Codex", Status: "working", SessionID: "native-b"}
	cur = []AgentStatus{a, b}
	h.tick()
	h.tick()
	if len(alerts) != 0 {
		t.Fatalf("unchanged native sessions alerted: %+v", alerts)
	}
	if len(h.prev) != 2 {
		t.Fatalf("hub tracks %d rows, want 2: %+v", len(h.prev), h.prev)
	}

	// b finishes: one done, for b, with no pane to name.
	b.Status = "idle"
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 1 || alerts[0].Kind != "done" || alerts[0].Agent != "Codex" || alerts[0].Pane != "" {
		t.Fatalf("alerts = %+v, want one done for Codex", alerts)
	}

	// Both start waiting: each alerts once, and each is tracked for its own re-nudge.
	alerts = nil
	a.Status, b.Status = "waiting", "waiting"
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 2 {
		t.Fatalf("alerts = %+v, want one waiting alert each", alerts)
	}
	if len(h.waitAlertAt) != 2 {
		t.Fatalf("re-nudge clocks = %v, want one per session", h.waitAlertAt)
	}
}

// A tmux row is still its pane.
func TestAgentStatusKey(t *testing.T) {
	for _, tc := range []struct {
		a    AgentStatus
		want string
	}{
		{AgentStatus{PaneID: "%3"}, "%3"},
		{AgentStatus{PaneID: "%3", SessionID: "s"}, "%3"},
		{AgentStatus{SessionID: "s"}, "native:s"},
		{AgentStatus{}, ""},
	} {
		if got := tc.a.key(); got != tc.want {
			t.Errorf("%+v.key() = %q, want %q", tc.a, got, tc.want)
		}
	}
}
