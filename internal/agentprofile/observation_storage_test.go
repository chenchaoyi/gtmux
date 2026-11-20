package agentprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestProfileObservationLeaseShardsAreBounded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const sampleSessions = 768
	shards := map[string]bool{}
	for i := range sampleSessions {
		session := fmt.Sprintf("profile-%d", i)
		shards[filepath.Base(observationPath("claude", session, "fixture"))[:2]] = true
		Observe("claude", session, "fixture", "Stop", []byte(`{"model":"observed"}`), false)
		p := New("claude", session, false)
		Current(&p, "fixture")
		if values(p)["model"] != "observed" {
			t.Fatalf("missing observation for %s", session)
		}
	}
	root := filepath.Join(state.Dir(), "participant-profiles", "v2")
	locks, err := os.ReadDir(filepath.Join(root, "locks"))
	if err != nil || len(locks) != len(shards) || len(locks) > 256 || len(locks) >= sampleSessions {
		t.Fatalf("shards=%d locks=%d err=%v", len(shards), len(locks), err)
	}
	legacyLocks, err := filepath.Glob(filepath.Join(state.Dir(), "lease-profile-*.lock"))
	if err != nil || len(legacyLocks) != 0 {
		t.Fatalf("new writes created legacy locks: %v, %v", legacyLocks, err)
	}
}

func TestProfileObservationIsolatesMixedWriters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const agent, session, home = "claude", "transition", "fixture"
	legacy := observationPath(agent, session, home)
	current := versionedObservationPath(legacy)
	writeLegacy := func(snapshot observation) []byte {
		t.Helper()
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := state.WritePrivateFile(legacy, data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	legacySnapshot := observation{Agent: agent, Session: session, Home: home, Facts: []Fact{
		fact("model", "old", "native_hook", 10), fact("effort", "old-effort", "native_hook", 10),
		fact("permission", "plan", "native_hook", 10), fact("mode", "plan", "native_hook", 10),
	}}
	legacyData := writeLegacy(legacySnapshot)
	legacyLease, err := state.TryLease("profile-" + filepath.Base(legacy))
	if err != nil {
		t.Fatal(err)
	}
	defer legacyLease.Close()

	p := New(agent, session, false)
	Current(&p, home)
	if values(p)["effort"] != "old-effort" || values(p)["mode"] != "plan" {
		t.Fatalf("legacy profile unavailable: %+v", p.Facts)
	}
	Observe(agent, session, home, "Stop", []byte(`{"to_model":"new","permission_mode":"default","effort":{"level":"high"}}`), false)
	if data, err := os.ReadFile(legacy); err != nil || !bytes.Equal(data, legacyData) {
		t.Fatalf("new writer changed old data: %v", err)
	}
	p = New(agent, session, false)
	Current(&p, home)
	if values(p)["model"] != "new" || values(p)["effort"] != "high" || values(p)["mode"] != "" {
		t.Fatalf("stale model options or mode: %+v", p.Facts)
	}

	shard, err := state.TryLeaseIn(filepath.Join(filepath.Dir(current), "locks"), filepath.Base(current)[:2])
	if err != nil {
		t.Fatal(err)
	}
	defer shard.Close()
	Observe(agent, session, home, "Stop", []byte(`{"model":"contended"}`), false)
	p = New(agent, session, false)
	Current(&p, home)
	if values(p)["model"] != "new" {
		t.Fatal("a contending writer acquired the shard")
	}
	if _, err := os.Stat(filepath.Join(state.Dir(), "lease-profile-"+filepath.Base(legacy)+".lock")); err != nil {
		t.Fatalf("the old lock was removed: %v", err)
	}
	if _, err := os.Stat(observationPath(agent, session, "foreign")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected foreign record: %v", err)
	}
	foreign := New(agent, session, false)
	Current(&foreign, "foreign")
	if len(foreign.Facts) != 0 {
		t.Fatal("a foreign identity read the observation")
	}

	newAt := observationFact(p.Facts, "model").At
	legacySnapshot.Facts = []Fact{
		fact("model", "late", "native_hook", newAt+1),
		fact("version", "legacy-version", "native_hook", newAt+1),
		fact("permission", "plan", "native_hook", 10), fact("mode", "plan", "native_hook", 10),
	}
	writeLegacy(legacySnapshot)
	p = New(agent, session, false)
	Current(&p, home)
	if got := values(p); got["model"] != "late" || got["version"] != "legacy-version" || got["permission"] != "default" ||
		got["effort"] != "" || got["mode"] != "" {
		t.Fatalf("late legacy write hid unrelated facts or restored old options: %+v", p.Facts)
	}
}
