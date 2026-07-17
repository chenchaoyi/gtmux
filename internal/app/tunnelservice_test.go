package app

import (
	"os"
	"path/filepath"
	"testing"
)

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
