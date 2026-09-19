package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func put(t *testing.T, p, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(p, mode)
}

// A clean, private home reads as healthy.
func TestDoctorLogsOnACleanHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	day := now.Format("2006-01-02")
	put(t, filepath.Join(state.LogsDir(), day+".jsonl"),
		`{"ts":"`+now.Format("2006-01-02T15:04:05.000Z07:00")+`","level":"info","component":"serve","kind":"diag","event":"serve.start"}`+"\n", 0o600)
	_ = os.Chmod(state.Dir(), 0o700)
	_ = os.Chmod(state.LogsDir(), 0o700)
	for _, r := range logsChecks(now) {
		if r.status == stRec || r.status == stMiss {
			t.Errorf("%s is flagged on a clean home: %s (%s)", r.label, r.value, r.note)
		}
	}
}

// Each thing that can go wrong is its own flagged row, and --fix clears them.
func TestDoctorLogsFlagsWhatWentWrongAndFixClearsIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	ts := now.Format("2006-01-02T15:04:05.000Z07:00")
	day := now.Format("2006-01-02")
	logs := state.LogsDir()
	put(t, filepath.Join(logs, now.AddDate(0, 0, -45).Format("2006-01-02")+".jsonl"), "{}\n", 0o600) // cleanup stopped
	put(t, filepath.Join(logs, day+".jsonl"),
		`{"ts":"`+ts+`","level":"warn","component":"log","kind":"diag","event":"log.runaway","attrs":{"component":"tunnel","event":"tunnel.probe.failed"}}`+"\n"+
			`{"ts":"`+ts+`","level":"error","component":"serve","kind":"diag","event":"handler.panic"}`+"\n", 0o600)
	put(t, filepath.Join(state.Dir(), "events.jsonl"), "x", 0o644)                   // world-readable
	put(t, filepath.Join(state.Dir(), "hq-feed", "spool.jsonl"), "x", 0o600)         // retired
	put(t, filepath.Join(state.ConfigDir(), "devices.json.bak-cleanup"), "x", 0o600) // credential copy

	flagged := map[string]bool{}
	for _, r := range logsChecks(now) {
		flagged[r.label] = r.status == stRec
	}
	for _, label := range []string{"log store", "runaway writer", "recent errors", "file modes", "other stores", "credential backups"} {
		if !flagged[label] {
			t.Errorf("%q is not flagged", label)
		}
	}

	s := &fixState{yes: true}
	if s.stepHousekeep() != 1 || s.stepCredentialBackups() != 1 {
		t.Fatal("--fix did not run the two steps")
	}
	after := map[string]bool{}
	for _, r := range logsChecks(now) {
		after[r.label] = r.status == stRec
	}
	for _, label := range []string{"log store", "file modes", "other stores"} {
		if after[label] {
			t.Errorf("%q is still flagged after --fix", label)
		}
	}
	if _, present := after["credential backups"]; present {
		t.Error("the credential backups row still shows after --fix removed them")
	}
}
