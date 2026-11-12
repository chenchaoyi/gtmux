package knowledge

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func receiptFixture() UsageReceipt {
	return UsageReceipt{Version: 1, TaskKey: strings.Repeat("a", 64), PlanID: strings.Repeat("b", 64), Purpose: "task", Stage: "shown", Observation: "self_report", Items: []UsageItem{{ID: "pitfalls/write-check", Revision: strings.Repeat("c", 64), ContentHash: strings.Repeat("d", 64)}}}
}

func TestUsageDoesNotBecomeHitAndRetryIsIdempotent(t *testing.T) {
	reviewFixture(t)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	r := receiptFixture()
	first, err := RecordUsage(r)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RecordUsage(r)
	if err != nil || again.ID != first.ID {
		t.Fatal("retry", err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("usage changed knowledge")
	}
	r.Stage = "outcome"
	r.Outcome = "succeeded"
	r.Purpose = "evaluation"
	if _, err := RecordUsage(r); err != nil {
		t.Fatal(err)
	}
	s, err := SummarizeUsage(0)
	if err != nil || s.Reports != 1 || s.Tasks != 1 || s.Stages["shown"] != 1 || s.EvaluationReports != 1 || s.CausalUseRate != nil {
		t.Fatalf("misleading summary: %+v %v", s, err)
	}
}
func TestUsageRejectsAuditAndForgedMeasurement(t *testing.T) {
	r := receiptFixture()
	r.Purpose = "audit"
	if err := validateUsage(r); err == nil {
		t.Fatal("audit became use")
	}
	r = receiptFixture()
	r.Observation = "measured"
	if err := validateUsage(r); err == nil {
		t.Fatal("caller claimed trusted measurement")
	}
	r = receiptFixture()
	r.TaskKey = "private task prose"
	if err := validateUsage(r); err == nil {
		t.Fatal("stored task text")
	}
	r = receiptFixture()
	r.Stage = "outcome"
	if err := validateUsage(r); err == nil {
		t.Fatal("missing outcome accepted")
	}
}
