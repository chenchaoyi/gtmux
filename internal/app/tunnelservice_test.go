package app

import (
	"context"
	"errors"
	"github.com/chenchaoyi/gtmux/internal/userservice"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectServiceStatusAndDefaultBackend(t *testing.T) {
	useLaunchdFixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(userServicesDir(), 0o755); err != nil {
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
	useLaunchdFixture(t)
	old := serviceManager
	serviceManager = func() userservice.Manager {
		return userservice.Launchd{Home: homeDir(), Run: func(context.Context, string, ...string) ([]byte, error) { return nil, nil }}
	}
	t.Cleanup(func() { serviceManager = old })
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

func useLaunchdFixture(t *testing.T) {
	t.Helper()
	old := serviceManager
	serviceManager = func() userservice.Manager { return userservice.Launchd{Home: homeDir(), Run: userservice.Command} }
	t.Cleanup(func() { serviceManager = old })
}

func TestUnavailableUserManagerDoesNotProvisionOrWriteCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	old := serviceManager
	serviceManager = func() userservice.Manager {
		return userservice.Systemd{Home: home, Run: func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("unavailable") }}
	}
	t.Cleanup(func() { serviceManager = old })
	if rc := serveServiceInstall(8765); rc == 0 {
		t.Fatal("unavailable manager reported success")
	}
	if fileExists(serveAgentPath()) || fileExists(filepath.Join(home, ".config", "gtmux", "serve-token")) {
		t.Fatal("failed preflight changed remote state")
	}
}

func TestFailedServiceStopRetainsArtifactsAndURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	old := serviceManager
	serviceManager = func() userservice.Manager {
		return userservice.Systemd{Home: home, Run: func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("refused") }}
	}
	t.Cleanup(func() { serviceManager = old })
	if err := os.MkdirAll(userServicesDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serveAgentPath(), []byte("existing unit"), 0600); err != nil {
		t.Fatal(err)
	}
	writeTunnelURL("https://retained.example")
	if serviceRemoveAll() == 0 {
		t.Fatal("stop failure reported disabled")
	}
	if !fileExists(serveAgentPath()) || readTunnelURL() != "https://retained.example" {
		t.Fatal("lost settings for a still-running service")
	}
}
