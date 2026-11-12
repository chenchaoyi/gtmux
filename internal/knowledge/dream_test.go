package knowledge

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDreamBudgetUnknownExhaustedAndFingerprint(t *testing.T) {
	e := cardFixture("release", "local")
	e.Memory.ReviewAfter = 50
	ops := []knowledgeOp{e}
	before, _ := json.Marshal(ops)
	q := DreamRequest{Mode: "routine", Limit: 20}
	p, err := dreamPlan(ops, q, 100)
	if err != nil || p.Budget.CanStart || p.Budget.UsageKnown {
		t.Fatalf("unknown usage: %+v %v", p, err)
	}
	q.UsageKnown = true
	p2, err := dreamPlan(ops, q, 100)
	if err != nil || !p2.Budget.CanStart || p.ID != p2.ID {
		t.Fatalf("fingerprint/budget: %+v %v", p2, err)
	}
	q.GenerationSpent = 400000
	p3, err := dreamPlan(ops, q, 100)
	if err != nil || p3.Budget.CanStart || p3.Budget.GenerationRemaining != 0 {
		t.Fatalf("exhausted: %+v %v", p3, err)
	}
	q.AllowBurst = true
	p4, err := dreamPlan(ops, q, 100)
	if err != nil || !p4.Budget.CanStart || p4.Budget.GenerationLimit != 1200000 {
		t.Fatalf("explicit burst: %+v %v", p4, err)
	}
	after, _ := json.Marshal(ops)
	if !bytes.Equal(before, after) {
		t.Fatal("dream changed source")
	}
}
func TestDreamPacketExcludesPrivateAndRejectsMissingIDs(t *testing.T) {
	a := cardFixture("release", "local")
	a.Memory = nil
	b := cardFixture("private", "local")
	b.Sensitive = true
	p, err := dreamPlan([]knowledgeOp{a, b}, DreamRequest{Mode: "pilot", UsageKnown: true}, 100)
	if err != nil || len(p.Items) != 1 || p.Excluded["private"] != 1 {
		t.Fatalf("privacy: %+v %v", p, err)
	}
	if _, err := dreamPlan([]knowledgeOp{a}, DreamRequest{Mode: "pilot", IDs: []string{"missing"}}, 100); err == nil {
		t.Fatal("missing selection silently ignored")
	}
	p, err = dreamPlan([]knowledgeOp{a}, DreamRequest{Mode: "pilot", ReviewedRevisions: map[string]string{a.ID: p.Items[0].Revision}}, 100)
	if err != nil || len(p.Items) != 0 || p.Excluded["already_reviewed_revision"] != 1 {
		t.Fatalf("repeated unchanged review: %+v %v", p, err)
	}
}
