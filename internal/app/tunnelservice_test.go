package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectServiceStatusAndDefaultBackend(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(launchAgentsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{serveAgentPath(), selfTunnelAgentPath()} {
		if err := os.WriteFile(path, []byte("<plist/>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if !serviceInstalled() || serviceBackend("") != "self" || serviceBackend("cloudflare") != "cloudflare" {
		t.Fatal("the installed Direct service must be visible and preserved unless Standard is explicit")
	}
	// A fake launchctl reports the two Direct agents loaded. No real user service
	// is touched by this test.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "launchctl"), []byte("#!/bin/sh\n[ \"$1\" = list ] && [ \"$2\" != com.gtmux.tunnel ]\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	writeTunnelURL("https://direct.example/p12345")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	if code := tunnelServiceStatus(); code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	_ = w.Close()
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if !strings.Contains(string(got), "Direct") || !strings.Contains(string(got), "https://direct.example/p12345") || strings.Contains(string(got), "off") {
		t.Fatalf("Direct status is wrong: %s", got)
	}
	if !alreadyServingTunnel() {
		t.Fatal("a loaded Direct service must prevent a second foreground tunnel")
	}
	if err := os.Remove(selfTunnelAgentPath()); err != nil {
		t.Fatal(err)
	}
	if serviceInstalled() || serviceBackend("") != "" {
		t.Fatal("a fresh install must still default to Standard")
	}
}

// serviceRemoveAll (the menu-bar "Off" / `serve --unservice`) must remove ALL
// remote-access agents — including the SELF-HOSTED (Direct) tunnel. When it skipped
// com.gtmux.selftunnel, turning Off while on the Direct backend left that agent
// running, so groundTruth still read .anywhere and the picker snapped back.
func TestServiceRemoveAllDropsSelfTunnel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "Library", "LaunchAgents"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{serveAgentPath(), tunnelAgentPath(), selfTunnelAgentPath()}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("<plist/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	serviceRemoveAll()

	for _, p := range paths {
		if fileExists(p) {
			t.Errorf("serviceRemoveAll left %s — the Direct/self-tunnel agent must be removed too", filepath.Base(p))
		}
	}
}
