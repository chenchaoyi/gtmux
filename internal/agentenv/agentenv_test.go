package agentenv

import (
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

// Unset (no config, no env) defaults to OFF — never guess a proxy.
func TestDefaultOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_AGENT_PROXY", "")
	if Prefix() != "" {
		t.Errorf("unset should add nothing, got %q", Prefix())
	}
}

func TestOff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"off"}`)
	if Prefix() != "" {
		t.Errorf("off should add nothing, got %q", Prefix())
	}
}

// "on" applies the local proxy port EXPLICITLY (no probe).
func TestOn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"on"}`)
	want := "HTTPS_PROXY=http://127.0.0.1:7897 HTTP_PROXY=http://127.0.0.1:7897 "
	if Prefix() != want {
		t.Errorf("on = %q, want %q", Prefix(), want)
	}
	// A custom port.
	writeCfg(t, home, `{"agentProxy":"on","agentProxyPort":1080}`)
	if got := Prefix(); got != "HTTPS_PROXY=http://127.0.0.1:1080 HTTP_PROXY=http://127.0.0.1:1080 " {
		t.Errorf("on custom port = %q", got)
	}
}

func TestExplicitURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"http://10.0.0.1:8888"}`)
	want := "HTTPS_PROXY=http://10.0.0.1:8888 HTTP_PROXY=http://10.0.0.1:8888 "
	if Prefix() != want {
		t.Errorf("explicit = %q, want %q", Prefix(), want)
	}
}

// The env var overrides config (the per-network switch).
func TestEnvOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCfg(t, home, `{"agentProxy":"off"}`) // config says off…
	t.Setenv("GTMUX_AGENT_PROXY", "on")       // …but env says on → on wins
	if Active() != "http://127.0.0.1:7897" {
		t.Errorf("env should override config, got %q", Active())
	}
	t.Setenv("GTMUX_AGENT_PROXY", "off")
	writeCfg(t, home, `{"agentProxy":"on"}`) // config on, env off → off wins
	if Active() != "" {
		t.Errorf("env off should override config on, got %q", Active())
	}
}

// The removed "auto" (a legacy config value) degrades to OFF — never the old probe.
func TestLegacyAutoIsOff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"auto"}`)
	if Prefix() != "" {
		t.Errorf("legacy 'auto' must degrade to off (no probe), got %q", Prefix())
	}
}

func TestWrapNoDouble(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"http://p:1"}`)
	pre := "HTTPS_PROXY=http://x claude"
	if Wrap(pre) != pre {
		t.Errorf("Wrap doubled a proxied command: %q", Wrap(pre))
	}
	if Wrap("claude") != "HTTPS_PROXY=http://p:1 HTTP_PROXY=http://p:1 claude" {
		t.Errorf("Wrap = %q", Wrap("claude"))
	}
}
