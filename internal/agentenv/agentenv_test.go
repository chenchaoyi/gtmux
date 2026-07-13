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

// Unset (no config, no env) → no proxy. gtmux never guesses.
func TestDefaultNone(t *testing.T) {
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

// A configured proxy URL is applied verbatim (the tool hard-codes no port/host).
func TestExplicitURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	writeCfg(t, home, `{"agentProxy":"http://127.0.0.1:7897"}`)
	want := "HTTPS_PROXY=http://127.0.0.1:7897 HTTP_PROXY=http://127.0.0.1:7897 "
	if Prefix() != want {
		t.Errorf("explicit = %q, want %q", Prefix(), want)
	}
}

// A non-URL value (incl. the removed "on"/"auto" keywords) means no proxy.
func TestNonURLIsNone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_AGENT_PROXY", "")
	for _, v := range []string{`"on"`, `"auto"`, `"true"`, `""`} {
		writeCfg(t, home, `{"agentProxy":`+v+`}`)
		if Prefix() != "" {
			t.Errorf("agentProxy=%s must be no proxy (only a URL applies), got %q", v, Prefix())
		}
	}
}

// The env var overrides config (the per-network switch).
func TestEnvOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCfg(t, home, `{"agentProxy":"off"}`)   // config says off…
	t.Setenv("GTMUX_AGENT_PROXY", "http://p:1") // …but env supplies a URL → it wins
	if Active() != "http://p:1" {
		t.Errorf("env should override config, got %q", Active())
	}
	t.Setenv("GTMUX_AGENT_PROXY", "off")
	writeCfg(t, home, `{"agentProxy":"http://p:1"}`) // config URL, env off → off wins
	if Active() != "" {
		t.Errorf("env off should override config URL, got %q", Active())
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
