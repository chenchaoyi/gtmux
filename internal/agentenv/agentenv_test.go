package agentenv

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, home, json string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

// listenEphemeral opens a real listener so "auto" detection sees a live port.
func listenEphemeral(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

func TestOff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCfg(t, home, `{"agentProxy":"off"}`)
	if Prefix() != "" {
		t.Errorf("off should add nothing, got %q", Prefix())
	}
}

func TestExplicit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCfg(t, home, `{"agentProxy":"http://10.0.0.1:8888"}`)
	want := "HTTPS_PROXY=http://10.0.0.1:8888 HTTP_PROXY=http://10.0.0.1:8888 "
	if Prefix() != want {
		t.Errorf("explicit = %q, want %q", Prefix(), want)
	}
}

func TestAutoDetectsListeningPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	port := listenEphemeral(t)
	writeCfg(t, home, `{"agentProxy":"auto","agentProxyPort":`+itoaT(port)+`}`)
	if Prefix() == "" {
		t.Error("auto should apply the proxy when the port is listening")
	}
	// A port nobody is on → nothing.
	writeCfg(t, home, `{"agentProxy":"auto","agentProxyPort":1}`)
	if Prefix() != "" {
		t.Errorf("auto with a dead port should add nothing, got %q", Prefix())
	}
}

func TestAutoIsDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // no config file at all → "auto", port 7897 (likely dead in CI)
	_ = Prefix()           // must not panic; result depends on whether 7897 listens
}

func TestWrapNoDouble(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCfg(t, home, `{"agentProxy":"http://p:1"}`)
	// already-proxied command is returned unchanged
	pre := "HTTPS_PROXY=http://x claude"
	if Wrap(pre) != pre {
		t.Errorf("Wrap doubled a proxied command: %q", Wrap(pre))
	}
	if Wrap("claude") != "HTTPS_PROXY=http://p:1 HTTP_PROXY=http://p:1 claude" {
		t.Errorf("Wrap = %q", Wrap("claude"))
	}
}

func itoaT(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
