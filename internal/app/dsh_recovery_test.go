package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestRestorePlanRecognizesVerifiedDSHOuterCLI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "work", "synthetic")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(`{"type":"session","version":4,"id":"synthetic","cwd":"/fixture/project"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	record := resume.Record{Agent: "dsh", TerminalClient: "dst", HomeID: dsh.HomeID(), SessionID: "synthetic", Cwd: "/fixture/project"}
	if err := resume.Save("research:0.1", record); err != nil {
		t.Fatal(err)
	}
	save := filepath.Join(t.TempDir(), "last")
	line := savePaneCmd("research", "0", "1", "/fixture/project", "node", ":node /fixture/.nvm/versions/node/v24.1.0/bin/dst")
	if err := os.WriteFile(save, []byte(line+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := buildRestorePlanFrom(save)
	if len(plan.Sessions) != 1 || len(plan.Sessions[0].Agents) != 1 || plan.Sessions[0].Agents[0].Agent != "dsh" || !plan.Sessions[0].Agents[0].Alive {
		t.Fatalf("verified terminal disappeared: %+v", plan)
	}
	for _, bad := range []resume.Record{
		{Agent: "dsh", TerminalClient: "dst", HomeID: "foreign", SessionID: "synthetic"},
		{Agent: "dsh", HomeID: dsh.HomeID(), SessionID: "synthetic"},
	} {
		if err := resume.Save("research:0.1", bad); err != nil {
			t.Fatal(err)
		}
		if p := buildRestorePlanFrom(save); len(p.Sessions[0].Agents) != 0 {
			t.Fatal("unverified record gained resume")
		}
	}
	if err := resume.Save("research:0.1", record); err != nil {
		t.Fatal(err)
	}
	for _, full := range []string{":bash", ":node /fixture/bin/dst --backend codex"} {
		if err := os.WriteFile(save, []byte(savePaneCmd("research", "0", "1", "/fixture/project", "node", full)+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if p := buildRestorePlanFrom(save); len(p.Sessions[0].Agents) != 0 {
			t.Fatal("idle shell or other backend gained resume")
		}
	}
}

func TestOwnedDSHRefreshPreservesProfilesAndSkipsVersionOnlyDrift(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	oldVersion := Version
	Version = "1.0.2"
	t.Cleanup(func() { Version = oldVersion })
	inst := agentInstallers["dsh"]
	if s, e := refreshDSHBridge(inst); e != nil || s != "not-installed" {
		t.Fatal(s, e)
	}
	if _, e := os.Stat(inst.configPath()); !os.IsNotExist(e) {
		t.Fatal("maintenance installed a missing bridge")
	}
	if err := updateDSHPlugin(inst, "/fixture/gtmux", true); err != nil {
		t.Fatal(err)
	}
	profile := dstProfilePath()
	if err := os.MkdirAll(filepath.Dir(profile), 0700); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("# user profile with custom YAML\n- custom: unchanged\n")
	if err := os.WriteFile(profile, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	previous := dshPluginForVersion("/fixture/gtmux", "1.0.1")
	if err := os.WriteFile(inst.configPath(), []byte(previous), 0600); err != nil {
		t.Fatal(err)
	}
	if !agentPluginCurrent(inst) {
		t.Fatal("version metadata alone made bridge stale")
	}
	if s, e := refreshDSHBridge(inst); e != nil || s != "current" {
		t.Fatal(s, e)
	}
	changed := strings.Replace(previous, "export const name = 'gtmux-dsh';", "export const name = 'old-bridge';", 1)
	if err := os.WriteFile(inst.configPath(), []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if s, e := refreshDSHBridge(inst); e != nil || s != "updated" {
		t.Fatal(s, e)
	}
	if b, _ := os.ReadFile(profile); string(b) != string(foreign) {
		t.Fatal("automatic refresh edited user profile")
	}
	if s, e := refreshDSHBridge(inst); e != nil || s != "current" {
		t.Fatal("not idempotent", s, e)
	}
	newer := strings.Replace(dshPluginForVersion("/fixture/gtmux", "1.0.3"), "export const name = 'gtmux-dsh';", "export const name = 'future-bridge';", 1)
	if err := os.WriteFile(inst.configPath(), []byte(newer), 0600); err != nil {
		t.Fatal(err)
	}
	if s, e := refreshDSHBridge(inst); e != nil || s != "newer" {
		t.Fatal("downgrade attempted", s, e)
	}
}

func TestOwnedDSHRefreshRefusesForeignFilesBeforeWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/fixture/gtmux", true); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(inst.configPath())
	if err := os.WriteFile(dshOverlayPath(inst), []byte("# user's overlay\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, e := refreshDSHBridge(inst); e == nil {
		t.Fatal("foreign overlay accepted")
	}
	after, _ := os.ReadFile(inst.configPath())
	if string(before) != string(after) {
		t.Fatal("wrote before ownership preflight")
	}
}

func TestRefreshOwnedHookCLIHasNoFreshInstallFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	var rc int
	out := captureStdout(t, func() { rc = cmdInstall([]string{"refresh-owned", "--agent", "dsh"}) })
	if rc != 0 || !strings.Contains(out, `"status":"not-installed"`) {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if rc := cmdInstall([]string{"refresh-owned", "--agent", "codex"}); rc != 2 {
		t.Fatal("refresh broadened to other agents")
	}
	if _, e := os.Stat(agentInstallers["dsh"].configPath()); !os.IsNotExist(e) {
		t.Fatal("fresh integration created")
	}
}

func TestOwnedDSHRefreshRefusesLinksAndConcurrentChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/fixture/gtmux", true); err != nil {
		t.Fatal(err)
	}
	target := dshOverlayPath(inst)
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "owned-looking-overlay")
	if err := os.WriteFile(outside, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshDSHBridge(inst); err == nil {
		t.Fatal("linked artifact accepted")
	}
	if after, _ := os.ReadFile(outside); string(after) != string(original) {
		t.Fatal("link target changed")
	}
	concurrent := filepath.Join(t.TempDir(), "bridge.mjs")
	if err := os.WriteFile(concurrent, []byte("user changed file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := refreshDSHArtifact(concurrent, []byte("previous"), []byte("replacement")); err == nil {
		t.Fatal("concurrent edit overwritten")
	}
	if after, _ := os.ReadFile(concurrent); string(after) != "user changed file" {
		t.Fatal("concurrent edit lost")
	}
}
