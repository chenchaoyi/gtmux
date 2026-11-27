package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

func TestPurposeDoctorFirstRead(t *testing.T) {
	for _, first := range []string{"cli", "menu"} {
		t.Run(first, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			for _, args := range [][]string{nil, {"--json"}, {"--no-prompt"}} {
				lease, ok := enterRuntimeLayout("doctor", args)
				if !ok || lease != nil {
					t.Fatal("first inspection created ordinary admission")
				}
			}
			if _, err := os.Lstat(state.Dir()); !os.IsNotExist(err) {
				t.Fatalf("inspection changed fresh root: %v", err)
			}
			if first == "menu" {
				if rc := Run([]string{"_storage-init"}); rc != 0 {
					t.Fatalf("menu initialization returned %d", rc)
				}
			} else {
				lease, ok := enterRuntimeLayout("agents", nil)
				if !ok || lease == nil {
					t.Fatal("first CLI writer refused")
				}
				defer lease.Close()
			}
			status, err := state.ReadStorageLayout()
			if err != nil || status.Mode != "purpose" {
				t.Fatalf("first writer selected %s: %v", status.Mode, err)
			}
		})
	}
}

func TestPurposeDoctorRegisteredCLI(t *testing.T) {
	f := newStorageLaunchdFixture(t)
	jobs, err := f.manager.Inspect()
	if err != nil || len(jobs) != 2 {
		t.Fatalf("registered programs: %v", err)
	}
	bin := filepath.Join(f.manager.Home, ".local", "bin", "gtmux")
	if err := os.MkdirAll(filepath.Dir(bin), 0700); err != nil {
		t.Fatal(err)
	}
	manager := userservice.StorageLaunchd{Home: f.manager.Home}
	for _, job := range jobs {
		if job.Program != bin {
			t.Fatalf("wrong launch target: %q", job.Program)
		}
	}
	for _, tc := range []struct {
		reply string
		want  bool
	}{{"gtmux 1.0.203", false}, {state.PurposeFormatCapability, true}} {
		script := fmt.Sprintf("#!/bin/sh\n[ \"$1:$2\" = 'version:--storage-format' ] || exit 2\nprintf '%%s\\n' '%s'\n", tc.reply)
		if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		for _, job := range jobs {
			err := manager.CheckPurposeExecutables([]userservice.StorageLaunchdJob{job})
			if (err == nil) != tc.want || err != nil && (!strings.Contains(err.Error(), job.Label) || !strings.Contains(err.Error(), bin)) {
				t.Fatalf("%s: %v", job.Label, err)
			}
		}
	}
}

func TestPurposeDoctorResumesPublishedGroup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := state.StorePath("events.jsonl")
	if err := state.WritePrivateFile(src, []byte("retained")); err != nil {
		t.Fatal(err)
	}
	stopped := func() error { return nil }
	if err := state.ChangeStorageLayout(true, stopped); err != nil {
		t.Fatal(err)
	}
	path := state.StorePath("events.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	entry := map[string]any{"Name": "events.jsonl", "Dev": uint64(stat.Dev), "Inode": uint64(stat.Ino), "Mode": 0}
	body, err := json.Marshal(map[string]any{"Version": 2, "Target": "grouped", "Entries": []any{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(filepath.Join(state.Dir(), ".storage-layout-transition.json"), body); err != nil {
		t.Fatal(err)
	}
	status, err := state.ReadStorageLayout()
	if err != nil || status.Mode != "grouped" || status.Continuing != "grouped" {
		t.Fatalf("wrong crash point: %+v %v", status, err)
	}
	if err := advancePurposePrerequisites(status, stopped); err != nil {
		t.Fatal(err)
	}
	if err := state.ChangePurposeLayout(true, stopped); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(state.StorePath("events.jsonl"))
	if err != nil || !os.SameFile(info, after) {
		t.Fatalf("lost original during recovery: %v", err)
	}
}

func TestPurposeDoctorOpenHandle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(state.Dir(), "terminal-input")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for _, tc := range []struct {
		body    string
		allowed bool
	}{
		{"printf 'p1234\\n'", false}, {"exit 1", true}, {"echo unavailable >&2; exit 1", false},
	} {
		if err := os.WriteFile(filepath.Join(bin, "lsof"), []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := checkPurposeOpenHandles(); (err == nil) != tc.allowed {
			t.Fatalf("open-handle admission: %v", err)
		}
	}
}
