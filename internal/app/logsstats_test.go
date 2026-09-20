package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// The menu bar reads --stats to decide whether to say "2 problems today", so the count
// has to be of the window asked for, not of the whole store.
func TestLogStatsCountsTheWindowAndReportsTheBounds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeStoreLine(t, "2026-09-18.jsonl", `{"ts":"2026-09-18T10:00:00.000+08:00","level":"error","component":"serve","kind":"diag","event":"serve.stop","msg":"older than the window"}`)
	writeStoreLine(t, "2026-09-19.jsonl", `{"ts":"2026-09-19T09:00:00.000+08:00","level":"info","component":"serve","kind":"diag","event":"serve.start","msg":"serve started"}`)
	writeStoreLine(t, "2026-09-19.jsonl", `{"ts":"2026-09-19T10:00:00.000+08:00","level":"warn","component":"serve","kind":"act","event":"act.pair","outcome":"refused","msg":"a pairing code was not accepted"}`)
	writeStoreLine(t, "2026-09-19.jsonl", `{"ts":"2026-09-19T11:00:00.000+08:00","level":"error","component":"tunnel","kind":"diag","event":"tunnel.down","msg":"the tunnel went away"}`)

	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.FixedZone("", 8*3600))
	since := time.Date(2026, 9, 19, 0, 0, 0, 0, time.FixedZone("", 8*3600))
	st := collectLogStats(state.LogsDir(), logsFilter{since: since, minLevel: diag.Debug}, now)

	if st.Entries != 3 {
		t.Errorf("entries in the window = %d, want 3", st.Entries)
	}
	if st.Warnings != 1 || st.Errors != 1 {
		t.Errorf("warnings/errors = %d/%d, want 1/1 (the older error is outside the window)", st.Warnings, st.Errors)
	}
	if st.Files != 2 || st.Oldest != "2026-09-18" {
		t.Errorf("store = %d files, oldest %q; want 2 and 2026-09-18", st.Files, st.Oldest)
	}
	if st.Bytes == 0 || st.RetainDays == 0 || st.MaxBytes == 0 {
		t.Errorf("bounds missing: %+v", st)
	}
	if st.WindowHours != 12 {
		t.Errorf("windowHours = %d, want 12", st.WindowHours)
	}
}

// An app reads the JSON form, so it stays a flat object with the names it was built on.
func TestLogStatsJSONKeepsItsShape(t *testing.T) {
	b, err := json.Marshal(logStats{Bytes: 7, Files: 1, RetainDays: 30, MaxBytes: 100, Entries: 2, Warnings: 1})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"bytes", "files", "retainDays", "maxBytes", "entries", "warnings", "errors"} {
		if _, ok := m[k]; !ok {
			t.Errorf("%q missing from %s", k, b)
		}
	}
}

// `gtmux config debug on` has to reach the launchd processes, which means config.json —
// and "on" has to mean every component, since someone turning it up does not yet know
// which part is at fault.
func TestConfigDebugWritesTheSwitchForEveryProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	read := func() string {
		b, err := os.ReadFile(filepath.Join(home, ".config", "gtmux", "config.json"))
		if err != nil {
			t.Fatalf("no config written: %v", err)
		}
		var c struct {
			Debug string `json:"debug"`
		}
		if err := json.Unmarshal(b, &c); err != nil {
			t.Fatal(err)
		}
		return c.Debug
	}

	if rc := configDebugKey([]string{"on"}); rc != 0 {
		t.Fatalf("config debug on = %d", rc)
	}
	if got := read(); got != "all" {
		t.Errorf("debug = %q after on, want all", got)
	}

	if rc := configDebugKey([]string{"off"}); rc != 0 {
		t.Fatalf("config debug off = %d", rc)
	}
	if got := read(); got != "" {
		t.Errorf("debug = %q after off, want empty", got)
	}

	if rc := configDebugKey([]string{"serve,tunnel"}); rc != 0 {
		t.Fatalf("config debug serve,tunnel = %d", rc)
	}
	if got := read(); got != "serve,tunnel" {
		t.Errorf("debug = %q, want serve,tunnel", got)
	}
}
