package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRenderSectionsTally checks the ok/recommended/blocking tally across status
// levels (stInfo must count toward none).
func TestRenderSectionsTally(t *testing.T) {
	secs := []dsection{{"x", []dcheck{
		{stOK, "a", "", ""},
		{stOK, "b", "", ""},
		{stRec, "c", "", ""},
		{stMiss, "d", "", ""},
		{stInfo, "e", "", ""},
	}}}
	ok, rec, miss := renderSections(secs)
	if ok != 2 || rec != 1 || miss != 1 {
		t.Fatalf("tally = ok %d rec %d miss %d, want 2/1/1", ok, rec, miss)
	}
}

// TestIsUTF8Locale covers the charset sniff used by rowLocale / stepLocale.
func TestIsUTF8Locale(t *testing.T) {
	for _, v := range []string{"en_US.UTF-8", "zh_CN.UTF-8", "C.utf8", "en_US.utf-8"} {
		if !isUTF8Locale(v) {
			t.Errorf("%q should be UTF-8", v)
		}
	}
	for _, v := range []string{"", "C", "POSIX", "en_US", "en_US.ISO8859-1"} {
		if isUTF8Locale(v) {
			t.Errorf("%q should not be UTF-8", v)
		}
	}
}

// TestLocaleCharsetPrecedence checks POSIX precedence (LC_ALL > LC_CTYPE > LANG)
// and that rowLocale flags a non-UTF-8 / unset locale as recommended, OK otherwise.
func TestLocaleCharsetPrecedence(t *testing.T) {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		t.Setenv(k, "")
	}
	if got := localeCharset(); got != "" {
		t.Fatalf("all unset → %q, want empty", got)
	}
	if rowLocale().status != stRec {
		t.Error("unset locale → recommended")
	}

	t.Setenv("LANG", "en_US.UTF-8")
	if got := localeCharset(); got != "en_US.UTF-8" {
		t.Fatalf("LANG only → %q", got)
	}
	if rowLocale().status != stOK {
		t.Error("UTF-8 LANG → ok")
	}

	t.Setenv("LC_ALL", "C") // LC_ALL wins over a UTF-8 LANG
	if got := localeCharset(); got != "C" {
		t.Fatalf("LC_ALL precedence → %q", got)
	}
	if rowLocale().status != stRec {
		t.Error("LC_ALL=C overrides UTF-8 LANG → recommended")
	}
}

// TestClaudeHookInstalled exercises the settings.json walk against a temp HOME:
// absent file, a non-gtmux hook, and a real gtmux hook command.
func TestClaudeHookInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if claudeHookInstalled() {
		t.Error("no settings.json → should report not installed")
	}

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath()

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/usr/bin/other thing"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if claudeHookInstalled() {
		t.Error("non-gtmux hook → should report not installed")
	}

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/opt/bin/gtmux hook"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !claudeHookInstalled() {
		t.Error("gtmux hook present → should report installed")
	}
}

// TestCodexNotifyIsGtmux: only a notify line referencing both gtmux and codex counts.
func TestCodexNotifyIsGtmux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if codexNotifyIsGtmux() {
		t.Error("no config.toml → not wired")
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".codex", "config.toml")

	if err := os.WriteFile(cfg, []byte(`notify = ["some-other-program"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if codexNotifyIsGtmux() {
		t.Error("unrelated notify → not wired")
	}

	if err := os.WriteFile(cfg, []byte(`notify = ["/opt/bin/gtmux", "hook", "--agent", "codex"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !codexNotifyIsGtmux() {
		t.Error("gtmux+codex notify → wired")
	}
}
