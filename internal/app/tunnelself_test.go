package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setDeviceID points HOME at a temp dir carrying a fixed tunnel-device-id, so the
// device-derived port is deterministic for the test.
func setDeviceID(t *testing.T, id string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tunnel-device-id"), []byte(id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// selfTunnelPort must be STABLE for a device (same across restarts) and land inside
// the VPS reverse-proxy's routable band — the range the Caddy `/p<port>` matcher
// accepts. Different device ids should (almost always) map to different ports, which
// is what makes Direct multi-tenant.
func TestSelfTunnelPort(t *testing.T) {
	setDeviceID(t, "aaaa1111bbbb2222cccc3333dddd4444")
	p1 := selfTunnelPort()
	if p1 < selfPortBase || p1 >= selfPortBase+selfPortSpan {
		t.Fatalf("port %d out of band [%d,%d)", p1, selfPortBase, selfPortBase+selfPortSpan)
	}
	if p1 < 20000 || p1 > 59999 {
		t.Fatalf("port %d outside the Caddy [2-5]\\d{4} matcher range", p1)
	}
	if p2 := selfTunnelPort(); p2 != p1 {
		t.Errorf("port not stable for one device: %d then %d", p1, p2)
	}

	setDeviceID(t, "9999zzzz8888yyyy7777xxxx6666wwww")
	if selfTunnelPort() == p1 {
		t.Errorf("distinct device ids collided on port %d (unexpected for these fixtures)", p1)
	}
}

func TestSelfTunnelPairURL(t *testing.T) {
	setDeviceID(t, "aaaa1111bbbb2222cccc3333dddd4444")
	got := selfTunnelPairURL("https://tunnel.ccy.dev")
	if !strings.HasPrefix(got, "https://tunnel.ccy.dev/p") {
		t.Fatalf("pair URL = %q, want a /p<port> path under the base", got)
	}
	// A trailing slash on the base must not produce a double slash.
	if got2 := selfTunnelPairURL("https://tunnel.ccy.dev/"); got2 != got {
		t.Errorf("trailing-slash base = %q, want %q (no //)", got2, got)
	}
}

// cmdSelfTunnelClient is the launchd service's entry (`gtmux tunnel-client`). With
// no Direct config it must exit non-zero and NOT block (selfTunnelConfig gates it),
// so a stray service invocation can't hang. Hermetic: an empty temp HOME + no env.
func TestSelfTunnelClientNoConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_SELFTUNNEL_URL", "")
	t.Setenv("GTMUX_SELFTUNNEL_SECRET", "")
	if got := cmdSelfTunnelClient([]string{"--port", "8765"}); got != 1 {
		t.Errorf("cmdSelfTunnelClient with no config = %d, want 1", got)
	}
}
