package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Installer unit tests never probe the developer's installed agents or home.
// Version-dependent tests provide an explicit version fixture.
func init() { profileVersionProbe = func(string) string { return "" } }

func TestProfileVersionProbeUsesBoundedKnownCLIOutput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "gemini")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n[ \"$1\" = --version ] || exit 2\nprintf '0.35.3\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if version := readProfileVersion("gemini"); version != "0.35.3\n" {
		t.Fatalf("known CLI version=%q", version)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 2\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if version := readProfileVersion("gemini"); version != "" {
		t.Fatalf("failed probe returned version=%q", version)
	}
	var prefix versionPrefix
	if n, err := prefix.Write(make([]byte, 4096)); err != nil || n != 4096 || len(prefix.b) != 256 {
		t.Fatal("unbounded version output")
	}
}

func TestProfileHookVersionGatesAndDowngrade(t *testing.T) {
	for _, tc := range []struct {
		agent, version string
		want           bool
	}{{"claude", "2.1.250", false}, {"claude", "2.1.251", true}, {"claude", "unknown", false}, {"gemini", "0.35.2", false}, {"gemini", "0.35.3", true}} {
		if supportsProfileHooks(tc.agent, tc.version) != tc.want {
			t.Fatal(tc)
		}
	}
	old := profileVersionProbe
	t.Cleanup(func() { profileVersionProbe = old })
	version := "2.1.303"
	profileVersionProbe = func(string) string { return version }
	path := filepath.Join(t.TempDir(), "settings.json")
	if e := updateSettings(path, testBin, true); e != nil {
		t.Fatal(e)
	}
	if g, _ := countHooks(t, path, "PostModelSwitch"); g != 1 {
		t.Fatal("supported model-switch hook missing")
	}
	version = "2.1.100"
	if e := updateSettings(path, testBin, true); e != nil {
		t.Fatal(e)
	}
	if g, _ := countHooks(t, path, "PostModelSwitch"); g != 0 {
		t.Fatal("old CLI kept unsupported hook")
	}
	version = "0.35.3"
	path = filepath.Join(t.TempDir(), "gemini.json")
	inst := agentInstallers["gemini"]
	if e := updateAgentSettings(inst, path, testBin, true); e != nil {
		t.Fatal(e)
	}
	if countAgentHooks(t, readSettings(t, path), "BeforeModel") != 1 {
		t.Fatal("model observer missing")
	}
	version = "0.35.2"
	if e := updateAgentSettings(inst, path, testBin, true); e != nil {
		t.Fatal(e)
	}
	if countAgentHooks(t, readSettings(t, path), "BeforeModel") != 0 {
		t.Fatal("old Gemini kept model observer")
	}
}
