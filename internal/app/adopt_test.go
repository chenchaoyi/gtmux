package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestAdoptSessionName(t *testing.T) {
	cases := map[string]string{
		"/Users/x/proj/acme-mobile": "acme-mobile",
		"/Users/x/my.proj":          "my-proj", // '.' → '-'
		"/Users/x/a b":              "a-b",     // space → '-'
		"/tmp/":                     "tmp",     // trailing slash
		"/":                         "",        // nothing usable
		"":                          "",
	}
	for cwd, want := range cases {
		if got := adoptSessionName(cwd); got != want {
			t.Errorf("adoptSessionName(%q) = %q, want %q", cwd, got, want)
		}
	}
}

func TestAdoptRejectsChatGPTDesktopBeforeSpawning(t *testing.T) {
	for _, originator := range []string{"codex_work_desktop", "Codex Desktop"} {
		t.Run(originator, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			codexHome := t.TempDir()
			t.Setenv("CODEX_HOME", codexHome)
			dir := filepath.Join(codexHome, "sessions", "2026", "09", "29")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			const id = "desktop-to-keep"
			data := `{"type":"session_meta","payload":{"id":"` + id + `","originator":"` + originator + `"}}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-29T00-00-00-"+id+".jsonl"), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := native.Save(native.Record{SessionID: id, Agent: "codex", State: "idle", UpdatedAt: time.Now().Unix()}); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "spawned")
			fakeTmux := filepath.Join(t.TempDir(), "tmux")
			if err := os.WriteFile(fakeTmux, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			saved := tmux.Bin
			tmux.Bin = fakeTmux
			t.Cleanup(func() { tmux.Bin = saved })
			if got := cmdAdopt([]string{id}); got != 1 {
				t.Fatalf("desktop adopt exit = %d, want refusal", got)
			}
			if _, ok := native.Load(id); !ok {
				t.Fatal("refused desktop session was removed")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("desktop refusal attempted to spawn tmux")
			}
		})
	}
}
