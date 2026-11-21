package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

func TestHWA005DeadlineIsOwedUntilOriginalMatterSettles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const due = 1700000000
	m := hqworkspace.Matter{
		ID: "synthetic-morning", State: "open",
		Responsibility: &hqworkspace.Responsibility{
			StepID: "report", Action: "Give the morning report", AuthorityRef: "synthetic-user",
			Hold: &hqworkspace.GoalHold{Kind: "external", NextCheck: due},
		},
	}
	d := hqworkspace.Empty()
	d.Matters[m.ID] = m
	prior := workspaceWakeDeadlineNotice
	t.Cleanup(func() { workspaceWakeDeadlineNotice = prior })
	var observed []hq.WakeDeadline
	workspaceWakeDeadlineNotice = func(_ int64, alerts []hq.WakeDeadline) { observed = alerts }
	workspaceWakeDeadlines(d, due+workspaceDeadlineGrace-1)
	if len(observed) != 0 {
		t.Fatal("deadline alarm preceded its grace window")
	}
	workspaceWakeDeadlines(d, due+workspaceDeadlineGrace)
	if len(observed) != 1 || observed[0] != (hq.WakeDeadline{FollowupID: hqworkspace.GoalFollowupID(m), MatterID: m.ID, Due: due}) {
		t.Fatalf("the scheduler lost the true deadline or its workbench identity: %#v", observed)
	}
	if d.Matters[m.ID].State != "open" {
		t.Fatal("notice changed workbench completion")
	}
	m.State = "completed"
	d.Matters[m.ID] = m
	workspaceWakeDeadlines(d, due+workspaceDeadlineGrace+1)
	if len(observed) != 0 {
		t.Fatal("completed goal still produced an overdue alarm")
	}
}
