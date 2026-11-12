package hq

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"golang.org/x/sys/unix"
)

func hqrHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	oldBin, oldLang := tmux.Bin, i18n.Lang()
	tmux.Bin = ""
	i18n.SetLang("en")
	t.Cleanup(func() { tmux.Bin = oldBin; i18n.SetLang(oldLang) })
	return home
}

func hqrPut(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func hqrSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			result[rel] = info.Mode().String()
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("%o:%d:%x", info.Mode().Perm(), info.ModTime().UnixNano(), sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func hqrRun(t *testing.T, want int, args ...string) {
	t.Helper()
	if got := CmdHQ(args); got != want {
		t.Fatalf("CmdHQ(%v) = %d, want %d", args, got, want)
	}
}

func TestHQR001RefreshOnlyUpdatesManagedRules(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			home := hqrHome(t)
			prior := playbookMarker(hqPlaybookVersion-1, lang) + "\noriginal managed text\n"
			hqrPut(t, hqInstructionsPath(), prior)
			for _, path := range []string{hqLocalPath(), hqClaudePointerPath(), filepath.Join(state.HQHome(), "notes/board.md"), filepath.Join(state.HQHome(), "knowledge/.ledger.jsonl"), filepath.Join(state.Dir(), "enrolled/%901"), filepath.Join(state.Dir(), "status/owner"), filepath.Join(state.Dir(), "input/pending")} {
				hqrPut(t, path, "unique fixture bytes: "+filepath.Base(path))
			}
			for i := 1; i <= 7; i++ {
				hqrPut(t, fmt.Sprintf("%s.bak-v%d-en", hqInstructionsPath(), i), fmt.Sprintf("old backup %d", i))
			}
			fake := t.TempDir()
			calls := filepath.Join(fake, "calls")
			script := filepath.Join(fake, "tmux")
			hqrPut(t, script, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+fmt.Sprintf("%q", calls)+"\nprintf '%%991\\n'\n")
			if err := os.Chmod(script, 0700); err != nil {
				t.Fatal(err)
			}
			tmux.Bin = script
			before := hqrSnapshot(t, home)
			hqrRun(t, 0, "--refresh-rules")
			body, err := os.ReadFile(hqInstructionsPath())
			if err != nil || string(body) != playbookIn(lang) {
				t.Fatalf("new rules differ from the shared renderer: %v", err)
			}
			backup := hqInstructionsPath() + fmt.Sprintf(".bak-v%d-%s", hqPlaybookVersion-1, lang)
			if b, err := os.ReadFile(backup); err != nil || string(b) != prior {
				t.Fatalf("exact prior text not retained: %v", err)
			}
			after := hqrSnapshot(t, home)
			instructions, _ := filepath.Rel(home, hqInstructionsPath())
			backupRel, _ := filepath.Rel(home, backup)
			lock, _ := filepath.Rel(home, filepath.Join(state.Dir(), ".hq-migration-apply.lock"))
			for path, old := range before {
				if path != instructions && after[path] != old {
					t.Errorf("unrelated existing file changed: %s", path)
				}
			}
			for path := range after {
				if _, existed := before[path]; !existed && path != backupRel && path != lock {
					t.Errorf("unexpected write outside rules/backup/lock: %s", path)
				}
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("refresh invoked tmux: possible enrollment, focus, launch or input")
			}
			tmux.Bin = ""
			hqrRun(t, 0, "--refresh-rules")
		})
	}
}

func TestHQR002RefreshKeepsLanguageAndConverges(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, version := range []int{hqPlaybookVersion - 1, hqPlaybookVersion, hqPlaybookVersion + 1} {
			t.Run(fmt.Sprintf("%s-v%d", lang, version), func(t *testing.T) {
				home := hqrHome(t)
				shellLang := "zh"
				if lang == "zh" {
					shellLang = "en"
				}
				i18n.SetLang(shellLang)
				original := playbookMarker(version, lang) + "\nretained current or future body\n"
				hqrPut(t, hqInstructionsPath(), original)
				hqrRun(t, 0, "--refresh-rules")
				body, err := os.ReadFile(hqInstructionsPath())
				want := original
				if version < hqPlaybookVersion {
					want = playbookIn(lang)
				}
				if err != nil || string(body) != want {
					t.Fatal("existing language or newer rules lost", err)
				}
				before := hqrSnapshot(t, home)
				hqrRun(t, 0, "--refresh-rules")
				if !reflect.DeepEqual(before, hqrSnapshot(t, home)) {
					t.Fatal("repeat changed bytes, mtimes or files")
				}
				for _, path := range []string{hqLocalPath(), hqClaudePointerPath(), filepath.Join(state.HQHome(), "notes"), filepath.Join(state.HQHome(), "knowledge")} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("refresh initialized optional content", path)
					}
				}
			})
		}
	}
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		if !strings.Contains(HelpText(80, false), "--refresh-rules") || !strings.Contains(HelpText(80, true), "--refresh-rules") {
			t.Fatal("normal/help-all does not expose rules-only maintenance")
		}
	}
	i18n.SetLang("en")
}

func TestHQR003RefreshRejectsConflictingOptions(t *testing.T) {
	for _, extra := range [][]string{{"--rotate"}, {"--here"}, {"--pane", "%991"}, {"--pane=%991"}, {"--new-pane"}, {"--agent", "codex"}, {"--lang", "zh"}, {"--board"}, {"--home"}, {"--records"}, {"--memory"}, {"--export", "unused"}, {"--import=unused"}, {"--plain"}, {"--passphrase-stdin"}, {"--expect-archive", "unused"}, {"--json"}, {"--brief-pane", "%991"}, {"--maintenance-done", "distill"}, {"--refresh-rules"}} {
		for _, first := range []bool{false, true} {
			t.Run(fmt.Sprint(extra, first), func(t *testing.T) {
				home := hqrHome(t)
				args := append([]string{"--refresh-rules"}, extra...)
				if !first {
					args = append(append([]string{}, extra...), "--refresh-rules")
				}
				hqrRun(t, 2, args...)
				if len(hqrSnapshot(t, home)) != 0 {
					t.Fatal("incompatible action touched the home")
				}
			})
		}
	}
}

func TestHQR004RefreshUsesExistingApplyLock(t *testing.T) {
	hqrHome(t)
	old := playbookMarker(hqPlaybookVersion-1, "en") + "\nold\n"
	hqrPut(t, hqInstructionsPath(), old)
	release, err := acquireMigrationApplyLock()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	hqrRun(t, 1, "--refresh-rules")
	if got, _ := os.ReadFile(hqInstructionsPath()); string(got) != old {
		t.Error("lock contention changed rules")
	}
	backups, _ := filepath.Glob(hqInstructionsPath() + ".bak-*")
	if len(backups) != 0 {
		t.Error("lock contention created backup")
	}
	release()
	hqrRun(t, 0, "--refresh-rules")
}

func TestHQR005RefreshRefusesUnsafeManagedFiles(t *testing.T) {
	for _, kind := range []string{"missing-home", "missing-file", "unmanaged", "embedded-marker", "unknown-language", "readonly", "symlink", "directory", "fifo", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			hqrHome(t)
			path := hqInstructionsPath()
			prior := playbookMarker(hqPlaybookVersion-1, "en") + "\nold\n"
			if kind != "missing-home" {
				if err := os.MkdirAll(state.HQHome(), 0700); err != nil {
					t.Fatal(err)
				}
			}
			outside := filepath.Join(t.TempDir(), "sentinel")
			hqrPut(t, outside, "external private fixture")
			switch kind {
			case "unmanaged":
				hqrPut(t, path, "personal unversioned rules")
			case "embedded-marker":
				hqrPut(t, path, "personal preface\n"+prior)
			case "unknown-language":
				hqrPut(t, path, playbookMarker(hqPlaybookVersion-1, "xx")+"\nold\n")
			case "readonly":
				hqrPut(t, path, prior)
				if err := os.Chmod(path, 0400); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				hqrPut(t, path, prior+strings.Repeat("x", 1024*1024))
			}
			var before map[string]string
			if kind != "missing-home" {
				before = hqrSnapshot(t, state.HQHome())
			}
			hqrRun(t, 1, "--refresh-rules")
			if kind == "missing-home" {
				if _, err := os.Stat(state.HQHome()); !os.IsNotExist(err) {
					t.Fatal("missing home was seeded")
				}
			} else if !reflect.DeepEqual(before, hqrSnapshot(t, state.HQHome())) {
				t.Fatal("unsafe source changed")
			}
			if got, _ := os.ReadFile(outside); string(got) != "external private fixture" {
				t.Fatal("external symlink target changed")
			}
		})
	}
}

func TestHQR006RefreshPreservesBackupNamesAndFailsSafely(t *testing.T) {
	for _, occupied := range []int{2, 100, -1} {
		t.Run(fmt.Sprint(occupied), func(t *testing.T) {
			hqrHome(t)
			prior := playbookMarker(hqPlaybookVersion-1, "en") + "\nold original\n"
			hqrPut(t, hqInstructionsPath(), prior)
			stem := hqInstructionsPath() + fmt.Sprintf(".bak-v%d-en", hqPlaybookVersion-1)
			for i := 1; i <= occupied; i++ {
				name := stem
				if i > 1 {
					name += fmt.Sprintf("-%d", i)
				}
				hqrPut(t, name, fmt.Sprintf("independent backup %d", i))
			}
			before := hqrSnapshot(t, state.HQHome())
			if occupied == -1 {
				if err := os.Chmod(state.HQHome(), 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(state.HQHome(), 0700) })
			}
			want := 1
			if occupied == 2 {
				want = 0
			}
			hqrRun(t, want, "--refresh-rules")
			after := hqrSnapshot(t, state.HQHome())
			for path, old := range before {
				if path == "AGENTS.md" && occupied == 2 {
					continue
				}
				if after[path] != old {
					t.Errorf("original or old backup changed: %s", path)
				}
			}
			if occupied == 2 {
				if b, err := os.ReadFile(stem + "-3"); err != nil || string(b) != prior {
					t.Fatal("collision did not retain original in fresh backup", err)
				}
			} else if !reflect.DeepEqual(before, after) {
				t.Fatal("backup failure changed HQ content")
			}
		})
	}
}
