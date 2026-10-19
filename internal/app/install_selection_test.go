package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

func TestAutoHookSelectionPreservesExplicitScopeAndDisabledPolicy(t *testing.T) {
	var calls [][]string
	run := func(args []string) int {
		calls = append(calls, args)
		if args[len(args)-1] == "dsh" {
			return 1
		}
		return 0
	}
	rc := installDetectedHooks([]string{"codex", "dsh", "opencode"}, []string{"--agent", "auto", "--yes"}, func(key string) bool { return key == "codex" }, run)
	want := [][]string{{"--yes", "--agent", "dsh"}, {"--yes", "--agent", "opencode"}}
	if rc != 1 || !reflect.DeepEqual(calls, want) {
		t.Fatalf("selection rc=%d calls=%v", rc, calls)
	}
	if rc := installDetectedHooks(nil, nil, func(string) bool { t.Fatal("empty selection asked policy"); return false }, run); rc != 0 {
		t.Fatal(rc)
	}
}

func TestDSHBinaryDetectionDoesNotEnableTUI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", "")
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	if err := os.WriteFile(filepath.Join(binDir, "dsh"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DSH_HOME", filepath.Join(homeDir(), "relocated-dsh"))
	if !agentHookPresent("dsh") {
		t.Fatal("installed DSH executable not detected")
	}
	m, _ := agents.For("dsh")
	if len(m.Detect) != 0 {
		t.Fatal("installation probe enabled TUI detection")
	}
	if _, err := os.Stat(filepath.Join(homeDir(), ".dsh")); !os.IsNotExist(err) {
		t.Fatal("detection created a profile")
	}
}

func TestExplicitDSHInstallUsesBundledWriter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	rc := 1
	out := captureStdout(t, func() { rc = cmdInstall([]string{"hooks", "--agent", "dsh", "--yes"}) })
	if rc != 0 || !agentHooksInstalled(agentInstallers["dsh"]) || !strings.Contains(out, "dsh web --patch") {
		t.Fatalf("install rc=%d output=%s", rc, out)
	}
	if _, err := os.Stat(claudeSettingsPath()); !os.IsNotExist(err) {
		t.Fatal("explicit DSH changed another agent")
	}
	if hookAgentArg(nil) != "claude" {
		t.Fatal("legacy default changed")
	}
	for key, inst := range agentInstallers {
		if inst.plugin != nil && inst.writePlugin == nil {
			t.Fatal("plugin writer missing:", key)
		}
	}
}
