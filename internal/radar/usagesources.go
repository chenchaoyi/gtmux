package radar

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/toolpath"
)

// UsageSource describes adapter coverage, not an inferred subscription.
// Registry executable probes establish installation only, never account ownership.
type UsageSource struct {
	Agent     string `json:"agent"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Tokens    bool   `json:"tokens"`
	Account   string `json:"account"` // quota, balance, or unsupported
	Status    string `json:"status,omitempty"`
}

func relevantUsageSources(rep UsageReport, panes []Pane) []UsageSource {
	return usageSources(rep, panes, toolpath.Look)
}
func usageSources(rep UsageReport, panes []Pane, look func(string) string) []UsageSource {
	observed := map[string]bool{}
	for _, p := range panes {
		key, _ := sessionRef(p)
		observed[key] = true
	}
	for _, r := range rep.History.ByAgent {
		observed[r.AgentKey] = true
	}
	for _, w := range rep.Limits.Windows {
		observed[w.Agent] = true
	}
	for _, b := range rep.Limits.Balances {
		observed[b.Agent] = true
	}
	var out []UsageSource
	for _, m := range agents.All() {
		installed := false
		for _, probe := range append(append([]string{}, m.Detect...), m.InstallDetect...) {
			if look(probe) != "" {
				installed = true
				break
			}
		}
		if !installed && !observed[m.Key] {
			continue
		}
		v := UsageSource{Agent: m.Key, Name: m.Label, Installed: installed, Account: "unsupported"}
		switch m.Key {
		case "claude", "codex":
			v.Tokens = true
			v.Account = "quota"
		case "dsh":
			v.Tokens = true
			v.Account = "balance"
			v.Status = rep.Limits.DSHAccountStatus
		}
		out = append(out, v)
	}
	return out
}
