package agentclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedLaunchSkipsLandingWithoutChangingClientIdentity(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NVM_DIR", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	cmd, ok := Command("dsh", "dst", "")
	if !ok || !strings.Contains(cmd, "'DSH_TUI_NO_LAUNCHPAD=1'") || !strings.Contains(cmd, "'dst' '--backend' 'dsh' '.'") {
		t.Fatal("fresh launch did not target a chat composer", cmd)
	}
	cmd, ok = Command("dsh", "dst", "synthetic")
	if !ok || !strings.Contains(cmd, "'--resume' 'synthetic'") {
		t.Fatal("resume lost target", cmd)
	}
	if _, ok := Command("dst", "dst", ""); ok {
		t.Fatal("client became its own harness")
	}
}

func TestSavedClientCannotRestoreAnotherBackendAsDSH(t *testing.T) {
	host := "node /profile/node_modules/@deepseek-harness-tui/dsh-tui/lib/types/dsh-adapter/host-entry.js"
	if !MatchesSavedClient("dst", host+" --backend dsh") {
		t.Fatal("DSH client liveness rejected")
	}
	for _, tail := range []string{" --backend codex", " --backend=claude"} {
		if MatchesSavedClient("dst", host+tail) {
			t.Fatal("other backend borrowed DSH restore identity")
		}
	}
}

func TestSavedDSTWrapperIsNotALiveAttachmentWitness(t *testing.T) {
	for _, cmd := range []string{"node /fixture/.nvm/versions/node/v24.1.0/bin/dst", "/fixture/bin/dst --backend dsh", "dst --backend=dsh"} {
		if !MatchesSavedClient("dst", cmd) {
			t.Fatal("outer CLI rejected", cmd)
		}
		if dstProcess(cmd) {
			t.Fatal("saved evidence became live input authority", cmd)
		}
		if MatchesSavedClient("dst", cmd+" --backend codex") {
			t.Fatal("other backend accepted", cmd)
		}
	}
	for _, cmd := range []string{"bash", "node /fixture/bin/other", "echo /fixture/bin/dst", "dst --backend=codex", "dst --backend"} {
		if MatchesSavedClient("dst", cmd) {
			t.Fatal("non-DSH evidence accepted", cmd)
		}
	}
}

func TestNVMClientLaunchIncludesItsNodeRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NVM_DIR", filepath.Join(home, ".nvm"))
	t.Setenv("PATH", t.TempDir())
	bin := filepath.Join(home, ".nvm", "versions", "node", "v24.1.0", "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"node", "dst"} {
		if err := os.WriteFile(filepath.Join(bin, n), []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	cmd, ok := Command("dsh", "dst", "synthetic")
	if !ok || !strings.Contains(cmd, "'"+filepath.Join(bin, "dst")+"'") || !strings.Contains(cmd, "'PATH="+bin+":") {
		t.Fatal("runtime missing", cmd)
	}
}
