package events

import "testing"

func TestReapResultCorrelationAndFailureRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, v := range []ReapContext{
		{TaskID: "task-a", Pane: "%7", Loc: "worker:0.0", Session: "worker", AgentSession: "thread-a", BindingID: "binding-a", ResourceClosed: true},
		{TaskID: "task-b", Pane: "%8", ResourceClosed: false, Failed: []string{"could not kill window"}},
		{TaskID: "task-c", Pane: "%9", ResourceClosed: true, Failed: []string{"could not delete branch"}},
	} {
		AuditReapResult(v, "closed owned resource", 100)
	}
	recs, _ := ReadSince(0)
	if len(recs) != 3 {
		t.Fatal(recs)
	}
	a := recs[0]
	if a.AgentSession != "thread-a" || a.BindingID != "binding-a" || a.TaskID != "task-a" || a.Loc != "worker:0.0" || a.OpID == "" || a.Outcome != "resource-closed" || a.ResourceClosed == nil || !*a.ResourceClosed {
		t.Fatal("unjoinable success", a)
	}
	for i, r := range recs {
		if !IsAudit(r) || !IsControl(r) || r.Severity != SevRoutine {
			t.Fatal("reap should not create fleet debt", r)
		}
		if i > 0 && (r.Outcome != "partial-or-failed" || r.ResourceClosed == nil || *r.ResourceClosed != (i == 2)) {
			t.Fatal("false success", r)
		}
	}
}
