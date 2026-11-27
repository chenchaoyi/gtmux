package app

import (
	"slices"
	"testing"
)

func agyDoctorCheckTitles() []string {
	var titles []string
	for _, c := range doctorAgentHookChecks() {
		titles = append(titles, c.label)
	}
	return titles
}

func TestDoctorOffersAgyHookOnlyWhenDiscoverable(t *testing.T) {
	hookPresenceHome(t)
	if !slices.Contains(installableHookAgentKeys(), "agy") {
		t.Fatalf("agy is not an installable hook agent: %v", installableHookAgentKeys())
	}
	if !slices.Contains(additionalHookAgents, "agy") {
		t.Fatalf("agy is missing from registry-derived doctor agents: %v", additionalHookAgents)
	}

	withHookExecutables(t)
	if slices.Contains(detectedHookAgents(), "agy") || slices.Contains(agyDoctorCheckTitles(), "Antigravity hook") {
		t.Fatal("undiscoverable agy must not be selected or checked")
	}

	withHookExecutables(t, "agy")
	if !slices.Contains(detectedHookAgents(), "agy") {
		t.Fatalf("discoverable agy not detected: %v", detectedHookAgents())
	}
	if !slices.Contains(agyDoctorCheckTitles(), "Antigravity hook") {
		t.Fatalf("doctor rows lack the Antigravity hook: %v", agyDoctorCheckTitles())
	}
	if row := rowAgentHook("agy"); row.status == stOK {
		t.Fatalf("unconfigured agy reported healthy: %+v", row)
	}

	s := &fixState{yes: true, bin: executableHookFixture(t)}
	if applied := s.stepAgentHook("agy"); applied != 1 || s.rc != 0 {
		t.Fatalf("doctor fix step applied=%d rc=%d", applied, s.rc)
	}
	if row := rowAgentHook("agy"); row.status != stOK {
		t.Fatalf("agy row after doctor --fix = %+v, want stOK", row)
	}
	if s.stepAgentHook("agy") != 0 {
		t.Fatal("a healthy agy hook must not be rewritten")
	}
}
