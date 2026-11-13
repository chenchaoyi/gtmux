package hq

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"golang.org/x/sys/unix"
)

func TestHQR007StandaloneTokenCannotBeConsumedAsAValue(t *testing.T) {
	for _, flag := range []string{"--agent", "--lang", "--pane", "--export", "--import", "--expect-archive", "--brief-pane", "--maintenance-done"} {
		t.Run(flag, func(t *testing.T) {
			home := hqrHome(t)
			calls := filepath.Join(t.TempDir(), "calls")
			bin := filepath.Join(t.TempDir(), "tmux")
			hqrPut(t, bin, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+fmt.Sprintf("%q", calls)+"\nexit 1\n")
			if err := os.Chmod(bin, 0700); err != nil {
				t.Fatal(err)
			}
			tmux.Bin = bin
			got := CmdHQ([]string{flag, "--refresh-rules"})
			body, _ := os.ReadFile(calls)
			if got != 2 || len(body) != 0 || len(hqrSnapshot(t, home)) != 0 {
				t.Fatalf("standalone token entered another action: exit=%d calls=%q files=%v", got, body, hqrSnapshot(t, home))
			}
		})
	}
}

func TestHQR008BackupKeepsReplacement(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			hqrHome(t)
			prior := playbookMarker(hqPlaybookVersion-1, "en") + "\nmanaged body\n"
			hqrPut(t, hqInstructionsPath(), prior)
			external := filepath.Join(t.TempDir(), "external")
			hqrPut(t, external, "replacement sentinel")
			previous := hqBackupWritten
			t.Cleanup(func() { hqBackupWritten = previous })
			path, moved := "", ""
			var replacement os.FileInfo
			hqBackupWritten = func(p string) {
				path, moved = p, p+".moved"
				if err := os.Rename(p, moved); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "file":
					hqrPut(t, p, "replacement sentinel")
				case "symlink":
					if err := os.Symlink(external, p); err != nil {
						t.Fatal(err)
					}
				case "fifo":
					if err := unix.Mkfifo(p, 0600); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				replacement, err = os.Lstat(p)
				if err != nil {
					t.Fatal(err)
				}
			}
			if got := CmdHQ([]string{"--refresh-rules"}); got != 1 {
				t.Fatalf("changed backup unexpectedly published: %d", got)
			}
			if path == "" {
				t.Fatal("backup creation boundary not reached")
			}
			current, err := os.Lstat(path)
			if err != nil || !os.SameFile(current, replacement) {
				t.Fatalf("replacement backup object removed or changed: %v", err)
			}
			if b, _ := os.ReadFile(moved); string(b) != prior {
				t.Fatal("created backup content lost")
			}
			if b, _ := os.ReadFile(hqInstructionsPath()); string(b) != prior {
				t.Fatal("failed backup changed original rules")
			}
			if b, _ := os.ReadFile(external); string(b) != "replacement sentinel" {
				t.Fatal("external replacement target modified")
			}
			if kind == "file" {
				if b, _ := os.ReadFile(path); string(b) != "replacement sentinel" {
					t.Fatal("replacement contents modified")
				}
			}
		})
	}
}

func TestHQR009MixedRulesAreNotSilentlyRebuilt(t *testing.T) {
	for _, tail := range []string{
		"<!-- user:personal -->\nKeep my instruction.\n<!-- /user:personal -->\n",
		"<!-- gtmux:knowledge:begin v1 c8ea7a7994de -->\nKeep machine knowledge.\n<!-- gtmux:knowledge:end -->\n",
		"Unmarked additional active instruction.\n",
	} {
		for _, position := range []string{"after", "after-space", "before"} {
			// Unmarked arbitrary text before LOCAL belongs to the generated body;
			// recognized independent blocks are protected in either position.
			if position == "before" && !strings.HasPrefix(tail, "<!--") {
				continue
			}
			t.Run(position+tail[:8], func(t *testing.T) {
				hqrHome(t)
				body := playbookMarker(hqPlaybookVersion-1, "en") + "\nmanaged body\n"
				if position == "before" {
					body += tail + "\n@LOCAL.md\n"
				} else {
					local := "@LOCAL.md"
					if position == "after-space" {
						local += " \t"
					}
					body += local + "\n" + tail
				}
				hqrPut(t, hqInstructionsPath(), body)
				before, _ := os.Stat(hqInstructionsPath())
				got := CmdHQ([]string{"--refresh-rules"})
				after, _ := os.Stat(hqInstructionsPath())
				now, _ := os.ReadFile(hqInstructionsPath())
				backups, _ := filepath.Glob(hqInstructionsPath() + ".bak-*")
				if got != 1 || string(now) != body || !after.ModTime().Equal(before.ModTime()) || len(backups) != 0 {
					t.Fatalf("mixed active instructions lost: exit=%d same=%v backups=%v", got, string(now) == body, backups)
				}
			})
		}
	}
}

func TestHQR010HardlinkedLockCannotChangeExternalPermissions(t *testing.T) {
	home := hqrHome(t)
	prior := playbookMarker(hqPlaybookVersion-1, "en") + "\nmanaged body\n"
	hqrPut(t, hqInstructionsPath(), prior)
	external := filepath.Join(home, "external-sentinel")
	if err := os.WriteFile(external, []byte("outside managed roots"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(external, 0644); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(state.Dir(), ".hq-migration-apply.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(external, lock); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(external)
	got := CmdHQ([]string{"--refresh-rules"})
	after, _ := os.Stat(external)
	body, _ := os.ReadFile(external)
	rules, _ := os.ReadFile(hqInstructionsPath())
	if got != 1 || after.Mode().Perm() != 0644 || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || string(body) != "outside managed roots" || string(rules) != prior {
		t.Fatalf("hardlinked lock changed external object: exit=%d mode=%o same=%v", got, after.Mode().Perm(), os.SameFile(before, after))
	}
}
