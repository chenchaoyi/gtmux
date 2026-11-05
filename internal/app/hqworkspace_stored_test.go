package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceStoredCLILeavesDiscoveryAndRecordsUntouched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	marker := filepath.Join(tools, "discovered")
	// A stored read must never invoke roster discovery, even if tools exist.
	for _, name := range []string{"tmux", "ps", "codex"} {
		script := "#!/bin/sh\nprintf called > '" + marker + "'\nexit 1\n"
		if err := os.WriteFile(filepath.Join(tools, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	store := workspaceStore()
	if _, err := store.Create("stored-goal", "Synthetic pending matter", 100); err != nil {
		t.Fatal(err)
	}
	before, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	code := 0
	wire := captureStdout(t, func() { code = cmdHQWorkspace([]string{"list", "--overview", "--stored", "--json"}) })
	var value workspaceList
	if code != 0 || json.Unmarshal([]byte(wire), &value) != nil || value.SourcesIncluded == nil || *value.SourcesIncluded || value.Overview == nil || len(value.Matters) != 1 || len(value.Conversations) != 0 || len(value.References) != 0 {
		t.Fatal("stored projection", code, wire)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("stored read discovered sessions", err)
	}
	after, err := store.Read()
	if err != nil || after.Revision != before.Revision || len(after.Messages) != len(before.Messages) {
		t.Fatal("read wrote records", err)
	}
	for _, args := range [][]string{{"show", "stored-goal", "--stored"}, {"list", "extra", "--stored"}, {"list", "--stored", "--stdin"}} {
		if got := cmdHQWorkspace(args); got != 2 {
			t.Fatal("invalid stored flag accepted", args, got)
		}
	}
}
