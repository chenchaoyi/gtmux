package agentprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestMixedProfileEffortResetSurvivesLegacyRefresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const agent, session, home = "claude", "same-model", "fixture"
	legacy := observationPath(agent, session, home)
	seed := observation{Agent: agent, Session: session, Home: home, Facts: HookFacts(agent,
		[]byte(`{"model":"same","effort":{"level":"high"}}`), 10)}
	writeLegacy := func(v observation) []byte {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := state.WritePrivateFile(legacy, data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	oldBytes := writeLegacy(seed)
	check := func(want string) {
		t.Helper()
		p := New(agent, session, false)
		Current(&p, home)
		if got := values(p); got["model"] != "same" || got["effort"] != want {
			t.Fatalf("model=%q effort=%q, want same/%q", got["model"], got["effort"], want)
		}
	}
	check("high")
	Observe(agent, session, home, "Stop", []byte(`{"model":"same","effort":{"level":""}}`), false)
	check("")
	if data, err := os.ReadFile(legacy); err != nil || !bytes.Equal(data, oldBytes) {
		t.Fatalf("new writer changed legacy bytes: %v", err)
	}
	current := readObservation(versionedObservationPath(legacy), agent, session, home)
	afterReset := observationFact(current.Facts, "model").At + 1000
	legacyLease, err := state.TryLease("profile-" + filepath.Base(legacy))
	if err != nil {
		t.Fatal(err)
	}
	defer legacyLease.Close()

	// The old observer starts from its own record. Its same-model path retains
	// options omitted from the hook payload and only advances observed facts.
	old := readObservation(legacy, agent, session, home)
	prior := New(agent, session, false)
	prior.Facts = old.Facts
	for _, f := range HookFacts(agent, []byte(`{"model":"same"}`), afterReset) {
		prior.put(f)
	}
	old.Facts = prior.Facts
	writeLegacy(old)
	if carried := observationFact(old.Facts, "effort"); carried.Value != "high" || carried.At != 10 {
		t.Fatalf("legacy fixture failed to carry its original effort: %+v", carried)
	}
	check("")

	for _, f := range HookFacts(agent, []byte(`{"model":"same","effort":{"level":"low"}}`), afterReset+1000) {
		prior.put(f)
	}
	old.Facts = prior.Facts
	writeLegacy(old)
	check("low")
}

func TestMixedProfileEffortResetPersistsWithoutOtherFacts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const agent, session, home = "claude", "only-effort", "fixture"
	legacy := observationPath(agent, session, home)
	v := observation{Agent: agent, Session: session, Home: home, Facts: HookFacts(agent, []byte(`{"effort":{"level":"high"}}`), 10)}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(legacy, data); err != nil {
		t.Fatal(err)
	}
	Observe(agent, session, home, "Stop", []byte(`{"effort":{"level":""}}`), false)
	p := New(agent, session, false)
	Current(&p, home)
	if effort := values(p)["effort"]; effort != "" {
		t.Fatalf("the final fact was restored after clearing effort: %q", effort)
	}
}
