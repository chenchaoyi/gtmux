package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestDoctorJSONHasCompleteBlockingReport(t *testing.T) {
	var out bytes.Buffer
	sections := []dsection{{"tools", []dcheck{{stOK, "tmux", "installed", ""}, {stMiss, "hook", "missing", "repair it"}, {stRec, "restore", "off", "optional"}, {stInfo, "note", "info", ""}}}}
	if rc := writeDoctorJSON(&out, sections, time.Unix(100, 0)); rc != 1 {
		t.Fatalf("blocking exit=%d", rc)
	}
	var r doctorReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Schema != 1 || r.CheckedAt != 100 || r.Version != Version || len(r.Sections) != 1 || len(r.Sections[0].Rows) != 4 || r.Counts["blocking"] != 1 || r.Counts["ok"] != 1 || r.Counts["recommended"] != 1 || r.Counts["info"] != 1 {
		t.Fatalf("incomplete report %+v", r)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("JSON contains terminal escapes")
	}
}
func TestDoctorReadOnlyRejectsMutationsBeforeChecks(t *testing.T) {
	for _, args := range [][]string{{"--json", "--fix"}, {"--json", "--yes"}, {"--json", "--bundle"}, {"--no-prompt", "--fix"}, {"--terminal"}, {"--terminal", "--fix", "--yes"}} {
		if got := cmdDoctor(args); got != 2 {
			t.Fatalf("%v exit=%d", args, got)
		}
	}
}
func TestPostUpdateDoctorUsesNewBinaryAndNeverPrompts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new gtmux")
	script := "#!/bin/sh\n[ \"$*\" = 'doctor --no-prompt --progress' ] || exit 9\nprintf 'new-binary-report'\nexit 1\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runPostUpdateDoctor(path, 10*time.Second, &out, io.Discard)
	if err == nil || out.String() != "new-binary-report" {
		t.Fatalf("wrong command or health result: %q %v", out.String(), err)
	}
	t.Setenv("GTMUX_BIN_DIR", filepath.Dir(path))
	if installedCLIPath() != filepath.Join(filepath.Dir(path), "gtmux") {
		t.Fatal("new install target ignored")
	}
}
func TestPostUpdateDoctorTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gtmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if runPostUpdateDoctor(path, 30*time.Millisecond, io.Discard, io.Discard) == nil || time.Since(start) > 3*time.Second {
		t.Fatal("unbounded post-update check")
	}
}

func TestDoctorJSONExplainsChecksWithoutChangingResults(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		var out bytes.Buffer
		sections := []dsection{{"tmux", []dcheck{{stRec, "history-limit", "50000", "history-limit=50000"}}}, {"agents", []dcheck{{stOK, "Codex hook", "installed", "config path"}}}, {"future", []dcheck{{stInfo, "unknown check", "custom", "original note"}}}}
		if code := writeDoctorJSON(&out, sections, time.Unix(100, 0)); code != 0 {
			t.Fatal(code)
		}
		var report doctorReport
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		row := report.Sections[0].Rows[0]
		if row.Label == "history-limit" || row.Description == "" || row.Status != "recommended" || row.Value != "50000" || row.Note != "history-limit=50000" {
			t.Fatalf("missing purpose or changed result: %+v", row)
		}
		hook := report.Sections[1].Rows[0]
		if !strings.Contains(hook.Description, "Codex") || hook.Status != "ok" {
			t.Fatalf("hook explanation: %+v", hook)
		}
		unknown := report.Sections[2].Rows[0]
		if unknown.Label != "unknown check" || unknown.Description != "" || unknown.Note != "original note" {
			t.Fatalf("invented explanation: %+v", unknown)
		}
	}
}
