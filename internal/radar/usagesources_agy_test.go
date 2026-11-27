package radar

import "testing"

func TestUsageSourcesDeclareAgyTokenCoverage(t *testing.T) {
	installed := func(name string) string {
		if name == "agy" {
			return "/fixture/bin/agy"
		}
		return ""
	}
	var agy *UsageSource
	sources := usageSources(UsageReport{}, nil, installed)
	for i := range sources {
		if sources[i].Agent == "agy" {
			agy = &sources[i]
		}
	}
	if agy == nil {
		t.Fatalf("installed agy missing from usage sources: %+v", sources)
	}
	if !agy.Installed || !agy.Tokens || agy.Account != "unsupported" || agy.Name != "Antigravity" {
		t.Fatalf("agy usage source = %+v, want installed token coverage without an account quota", *agy)
	}

	none := func(string) string { return "" }
	for _, s := range usageSources(UsageReport{}, nil, none) {
		if s.Agent == "agy" {
			t.Fatalf("uninstalled, unobserved agy must be omitted: %+v", s)
		}
	}
}
