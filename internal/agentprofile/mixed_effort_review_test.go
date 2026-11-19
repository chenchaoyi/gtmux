package agentprofile

import (
	"encoding/json"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestMixedProfileLegacyEffortOnlyAfterReset(t *testing.T) {
	for _, tt := range []struct {
		name, oldModel, newModel, laterEffort string
	}{
		{"no-model", "", "", "low"},
		{"same-model", "same", "same", "low"},
		{"different-models", "old", "new", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			const agent, session, home = "claude", "effort-only", "fixture"
			legacy := observationPath(agent, session, home)
			old := New(agent, session, false)
			input := func(model, effort string) []byte {
				t.Helper()
				fields := map[string]any{"effort": map[string]string{"level": effort}}
				if model != "" {
					fields["model"] = model
				}
				raw, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			writeLegacy := func(raw []byte, at int64) {
				t.Helper()
				for _, f := range HookFacts(agent, raw, at) {
					old.put(f)
				}
				data, err := json.Marshal(observation{Agent: agent, Session: session, Home: home, Facts: old.Facts})
				if err != nil {
					t.Fatal(err)
				}
				if err := state.WritePrivateFile(legacy, data); err != nil {
					t.Fatal(err)
				}
			}
			check := func(model, effort string) {
				t.Helper()
				p := New(agent, session, false)
				Current(&p, home)
				if got := values(p); got["model"] != model || got["effort"] != effort {
					t.Fatalf("model=%q effort=%q, want %q/%q", got["model"], got["effort"], model, effort)
				}
			}

			writeLegacy(input(tt.oldModel, "high"), 10)
			check(tt.oldModel, "high")
			Observe(agent, session, home, "Stop", input(tt.newModel, ""), false)
			check(tt.newModel, "")
			reset := readObservation(versionedObservationPath(legacy), agent, session, home).EffortResetAt
			if reset <= 10 {
				t.Fatalf("missing persisted reset: %d", reset)
			}
			writeLegacy(input("", "high"), reset)
			check(tt.newModel, "")
			originalModel := observationFact(old.Facts, "model")
			writeLegacy(input("", "low"), reset+1)
			if observationFact(old.Facts, "model") != originalModel {
				t.Fatal("the effort-only hook must leave the legacy model unchanged")
			}
			check(tt.newModel, tt.laterEffort)
		})
	}
}
