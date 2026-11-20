package agentprofile

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestMixedProfileModelReturnDoesNotRestoreEffort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const agent, session, home = "claude", "model-return", "fixture"
	legacy := observationPath(agent, session, home)
	Observe(agent, session, home, "Stop", []byte(`{"model":"a","effort":{"level":"high"}}`), false)
	current := readObservation(versionedObservationPath(legacy), agent, session, home)
	at := observationFact(current.Facts, "model").At
	if at == 0 {
		t.Fatal("missing initial current-generation observation")
	}

	lease, err := state.TryLease("profile-" + filepath.Base(legacy))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()

	old := New(agent, session, false)
	writeLegacy := func(raw string, when int64) {
		t.Helper()
		for _, f := range HookFacts(agent, []byte(raw), when) {
			if f.Key == "model" {
				if last := observationFact(old.Facts, "model"); last.Value != "" && last.Value != f.Value {
					old.Facts = slices.DeleteFunc(old.Facts, func(option Fact) bool { return modelOption(option.Key) })
				}
			}
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

	writeLegacy(`{"model":"a","effort":{"level":"high"}}`, at-1)
	check("a", "high")
	writeLegacy(`{"to_model":"b"}`, at+1)
	check("b", "")
	writeLegacy(`{"to_model":"a"}`, at+2)
	check("a", "")
	writeLegacy(`{"effort":{"level":"low"}}`, at+3)
	check("a", "low")
}
