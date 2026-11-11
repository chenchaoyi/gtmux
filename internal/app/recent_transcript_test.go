package app

import (
	"encoding/json"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"os"
	"path/filepath"
	"testing"
)

func TestLCL002RecentPaneKeepsOrdinalsCursorAndFullDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	const sid = "b29c685a-7a56-4c61-a735-ddf2b8fcd5f6"
	dir := filepath.Join(home, ".claude", "projects", "fixture")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	log := ""
	for i := 1; i <= 100; i++ {
		log += fmt.Sprintf("{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"question %d\"}}\n", i)
	}
	if e := os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(log), 0600); e != nil {
		t.Fatal(e)
	}
	if e := resume.Save("fixture:0.0", resume.Record{Agent: "claude", SessionID: sid}); e != nil {
		t.Fatal(e)
	}
	stub := filepath.Join(home, "tmux")
	if e := os.WriteFile(stub, []byte("#!/bin/sh\ncase \"$2\" in\n display-message) case \"$6\" in '#{pane_id}') echo '%1';; *) echo 'fixture:0.0';; esac;;\nesac\n"), 0700); e != nil {
		t.Fatal(e)
	}
	old := tmux.Bin
	tmux.Bin = stub
	t.Cleanup(func() { tmux.Bin = old })
	raw, meta, e := transcriptWindowForPane("%1", 0, false)
	var turns []transcript.Turn
	if e == nil {
		e = json.Unmarshal(raw, &turns)
	}
	if e != nil || len(turns) != 20 || turns[0].Ordinal != 81 || turns[0].HistoryBefore == "" || meta.Dropped != 80 {
		t.Fatal("recent window/cursor", len(turns), meta, e)
	}
	raw, full, e := transcriptForPane("%1", 0)
	var all []transcript.Turn
	if e == nil {
		e = json.Unmarshal(raw, &all)
	}
	if e != nil || len(all) != 100 || all[0].Ordinal != 1 || full.Etag == meta.Etag {
		t.Fatal("default history or validator changed", len(all), e)
	}
	raw, history, e := transcriptWindowForPane("%1", 0, true)
	if e != nil || string(raw) != "[]" || history.History == nil {
		t.Fatal("metadata-only history", string(raw), e)
	}
}
