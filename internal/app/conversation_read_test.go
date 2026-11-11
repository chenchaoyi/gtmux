package app

import (
	"encoding/base64"
	"errors"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"os"
	"path/filepath"
	"testing"
)

func TestCAS003PaneAssociationIsNotAnEmptyConversation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	stub := filepath.Join(t.TempDir(), "tmux")
	if e := os.WriteFile(stub, []byte("#!/bin/sh\ncase \"$6\" in '#{pane_id}') echo '%1';; *) echo 'fixture:0.0';; esac\n"), 0700); e != nil {
		t.Fatal(e)
	}
	old := tmux.Bin
	tmux.Bin = stub
	t.Cleanup(func() { tmux.Bin = old })
	check := func() {
		t.Helper()
		for _, history := range []bool{false, true} {
			_, _, e := transcriptWindowForPane("%1", 0, history)
			if !errors.Is(e, server.ErrConversationUnassociated) {
				t.Fatal("bad association disguised as empty history", e)
			}
		}
	}
	check()
	if e := resume.Save("fixture:0.0", resume.Record{Agent: "claude", SessionID: "empty"}); e != nil {
		t.Fatal(e)
	}
	b, _, e := transcriptForPane("%1", 0)
	if e != nil || string(b) != "[]" {
		t.Fatal("genuine empty changed", string(b), e)
	}
	// A source I/O failure is not an empty conversation either.
	logDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", "fixture", "empty.jsonl")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := transcriptForPane("%1", 0); !errors.Is(err, server.ErrConversationUnavailable) {
		t.Fatal("source read error swallowed", err)
	}
	if err := os.Remove(logDir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(resume.Dir(), base64.RawURLEncoding.EncodeToString([]byte("fixture:0.0"))+".json")
	if e := os.WriteFile(path, []byte(`{"agent":"claude","sessionId":"empty"} garbage`), 0600); e != nil {
		t.Fatal(e)
	}
	check()
	if e := resume.Save("fixture:0.0", resume.Record{Agent: "codex", SessionID: "stale", Frontend: "0123456789abcdef0123456789abcdef"}); e != nil {
		t.Fatal(e)
	}
	check()
}
