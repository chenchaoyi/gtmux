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
		ID: "synthetic-report", State: "open",
		Responsibility: &hqworkspace.Responsibility{
			StepID: "check", Action: "Check an external build", AuthorityRef: "synthetic-user",
			Hold: &hqworkspace.GoalHold{Kind: "external", NextCheck: due},
		},
	}
	d := hqworkspace.Empty()
	d.Matters[m.ID] = m
	prior := workspaceAttentionNotice
	t.Cleanup(func() { workspaceAttentionNotice = prior })
	var observed []hq.WorkspaceAttention
	workspaceAttentionNotice = func(_ int64, alerts []hq.WorkspaceAttention) { observed = alerts }

	for _, offset := range []int64{0, 60, 120} {
		m.Responsibility.Hold.NextCheck = due + offset
		d.Matters[m.ID] = m
		workspaceAttention(d, due+offset+120)
		if len(observed) != 0 || hqworkspace.GoalResponsibility(d, m, due+offset+120).State != "owes-hq" {
			t.Fatalf("routine recheck should stay visible without alerting: %#v", observed)
		}
	}
	if d.Matters[m.ID].State != "open" || m.Responsibility.Hold.NextCheck != due+120 {
		t.Fatal("desktop projection changed workbench obligation")
	}

	intent := hqworkspace.ActionIntent{ID: "action-current", Ref: "event:3", At: due}
	m.Responsibility.Hold, m.Responsibility.Intent = nil, &intent
	d.Matters[m.ID] = m
	attempt := hqworkspace.IntentAttempt{MatterID: m.ID, StepID: "old-step", Intent: intent, State: "held", At: due}
	d.IntentAttempts[intent.ID] = attempt
	workspaceAttention(d, due+120)
	if len(observed) != 0 {
		t.Fatal("a different step borrowed a stale attempt")
	}
	attempt.StepID = m.Responsibility.StepID
	d.IntentAttempts[intent.ID] = attempt
	workspaceAttention(d, due+workspaceAttentionGrace-1)
	if len(observed) != 0 {
		t.Fatal("transient refusal bypassed the grace period")
	}
	workspaceAttention(d, due+workspaceAttentionGrace)
	if len(observed) != 1 || observed[0] != (hq.WorkspaceAttention{MatterID: m.ID, EvidenceID: "action-current@held"}) {
		t.Fatalf("a continuing verified blocker lost its notice: %#v", observed)
	}
	m.State, m.UserInput = "needs-input", &hqworkspace.UserInput{ID: "question"}
	d.Matters[m.ID] = m
	workspaceAttention(d, due+120)
	if len(observed) != 1 || observed[0] != (hq.WorkspaceAttention{MatterID: m.ID, EvidenceID: "question", NeedsUser: true}) {
		t.Fatalf("a pending decision should supersede the earlier blocker: %#v", observed)
	}
	m.State = "completed"
	d.Matters[m.ID] = m
	workspaceAttention(d, due+240)
	if len(observed) != 0 {
		t.Fatal("completed matter still produced a desktop alert")
	}
}
