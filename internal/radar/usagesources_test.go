package radar

import (
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/usage"
	"testing"
)

func TestUsageSourcesFollowInstallationAndEvidenceWithoutInventingPlans(t *testing.T) {
	rep := UsageReport{History: usage.History{ByAgent: []usage.AgentHistory{{AgentKey: "dsh", WeekOut: 20}}}, Limits: limits.Report{Windows: []limits.Window{{Agent: "codex", PlanType: "pro"}}}}
	rows := usageSources(rep, nil, func(probe string) string {
		if probe == "gemini" {
			return "/tools/gemini"
		}
		return ""
	})
	by := map[string]UsageSource{}
	for _, r := range rows {
		by[r.Agent] = r
	}
	if by["dsh"].Installed || !by["dsh"].Tokens || by["dsh"].Account != "balance" {
		t.Fatal(rows)
	}
	if by["codex"].Installed || by["codex"].Account != "quota" {
		t.Fatal(rows)
	}
	if !by["gemini"].Installed || by["gemini"].Account != "unsupported" {
		t.Fatal(rows)
	}
	if _, ok := by["claude"]; ok {
		t.Fatal("unobserved uninstalled agent shown")
	}
}
