package hq

import (
	"strings"
	"testing"
)

// The shipped instruction is the integration surface: no worker launch, miner,
// watermark consumption or active-home reseed belongs in this contract test.
func TestKnowledgeWorkflowUsesBoundedReviewWithoutManufacturedReuse(t *testing.T) {
	for lang, text := range map[string]string{"en": hqInstructionsEN, "zh": hqInstructionsZH} {
		t.Run(lang, func(t *testing.T) {
			for _, required := range []string{
				"knowledge consult --query", "missing_required", "--role hq --budget 1200",
				"knowledge dream plan", "knowledge dream run", "knowledge dream history", "knowledge dream list", "reviewed_revisions", "knowledge review check",
				"knowledge review apply", "usage_known:false", "can_start:false", "self_report",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("missing workflow contract %q", required)
				}
			}
			for _, retired := range []string{
				"that gap is ITSELF a capture trigger", "这个空白本身就是沉淀触发点",
				"the carrier failed, not the memory", "是载体失效,不是\n  没记住",
			} {
				if strings.Contains(text, retired) {
					t.Errorf("retired causal/write incentive remains: %q", retired)
				}
			}
		})
	}
	if hqPlaybookVersion <= 62 {
		t.Fatal("new instructions would not reach an existing v62 home")
	}
}
