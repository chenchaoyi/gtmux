package hq

import (
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestHQHelpReadableAtDisplayWidths(t *testing.T) {
	defer i18n.SetLang("en")
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		for _, width := range []int{40, 60, 80} {
			for _, all := range []bool{false, true} {
				for _, line := range strings.Split(HelpText(width, all), "\n") {
					if i18n.DispWidth(line) > width {
						t.Errorf("%s width=%d all=%t: line too wide: %q", lang, width, all, line)
					}
				}
			}
		}
		common, full := HelpText(80, false), HelpText(80, true)
		for _, f := range HelpFlags() {
			if !strings.Contains(full, f.Name) && f.Name != "--help-all" {
				t.Errorf("full help lost %q", f.Name)
			}
			if f.section < 4 && !strings.Contains(common, f.Name) {
				t.Errorf("common help lost %q", f.Name)
			}
		}
		if strings.Contains(common, "--maintenance-done") || strings.Contains(common, "GTMUX_HQ_PASSPHRASE") {
			t.Fatal("internal details obscure common help")
		}
		if !strings.Contains(common, "gtmux hq --help-all") {
			t.Fatal("common help must point to full help")
		}
		if n := len(strings.Split(strings.TrimSpace(common), "\n")); n > 30 {
			t.Errorf("80-column common help is too long: %d lines", n)
		}
	}
}

func TestHQHelpDoesNotTouchHomeOrRotate(t *testing.T) {
	i18n.SetLang("en")
	t.Setenv("COLUMNS", "80")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	for _, args := range [][]string{{"--help"}, {"--rotate", "--help"}, {"--here", "--help-all"}} {
		out := captureStdout(t, func() {
			if rc := CmdHQ(args); rc != 0 {
				t.Errorf("help failed: %v rc=%d", args, rc)
			}
		})
		want := HelpText(80, args[len(args)-1] == "--help-all")
		if out != want {
			t.Errorf("help rendering differs for %v", args)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("help changed HQ home: %v, %v", entries, err)
	}
}

func TestMigrationHelpKeepsItsOwnEntryPoint(t *testing.T) {
	out := captureStdout(t, func() {
		if rc := CmdHQ([]string{"migrate", "--help"}); rc != 0 {
			t.Errorf("migration help rc=%d", rc)
		}
	})
	if !strings.Contains(out, "--from ARCHIVE") || strings.Contains(out, "--rotate") {
		t.Fatalf("migration help was replaced by main HQ help: %s", out)
	}
}
