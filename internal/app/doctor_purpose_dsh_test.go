package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestFreshDoctorDSHRepair(t *testing.T) {
	for _, first := range []string{"cli", "menu"} {
		t.Run(first, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("DSH_HOME", t.TempDir())
			inst := agentInstallers["dsh"]
			if err := updateDSHPlugin(inst, "/fixture/gtmux", true); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst", "bridge.mjs")
			if err := os.WriteFile(path, []byte(dshTerminalMarker+" prior owned fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, ok := enterRuntimeLayout("agents", nil); ok {
				t.Fatal("CLI admitted old bridge")
			}
			if rc := Run([]string{"_storage-init"}); rc == 0 {
				t.Fatal("menu admitted old bridge")
			}
			if rc := Run([]string{"install", "refresh-owned", "--agent", "dsh"}); rc != 0 {
				t.Fatal("advertised refresh failed")
			}
			if _, err := os.Stat(state.Dir()); !os.IsNotExist(err) {
				t.Fatalf("configuration refresh created storage: %v", err)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != dshTerminalPluginTemplate {
				t.Fatalf("bridge did not update: %v", err)
			}
			if first == "menu" {
				if rc := Run([]string{"_storage-init"}); rc != 0 {
					t.Fatal("menu refused new bridge")
				}
			} else {
				lease, ok := enterRuntimeLayout("agents", nil)
				if !ok {
					t.Fatal("CLI refused updated bridge")
				}
				defer lease.Close()
			}
			if s, err := state.ReadStorageLayout(); err != nil || s.Mode != "purpose" {
				t.Fatalf("first write: %s, %v", s.Mode, err)
			}
		})
	}
}

func TestFreshDoctorForeignDSH(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	p := filepath.Join(filepath.Dir(agentInstallers["dsh"].configPath()), "terminal-dst", "bridge.mjs")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("custom bridge"), 0600); err != nil {
		t.Fatal(err)
	}
	if rc := Run([]string{"install", "refresh-owned", "--agent", "dsh"}); rc == 0 {
		t.Fatal("refreshed an unknown terminal-only bridge")
	}
	if got, err := os.ReadFile(p); err != nil || string(got) != "custom bridge" {
		t.Fatalf("custom bridge changed: %v", err)
	}
	if _, err := os.Stat(state.Dir()); !os.IsNotExist(err) {
		t.Fatal("failure created a flat data root")
	}
}
