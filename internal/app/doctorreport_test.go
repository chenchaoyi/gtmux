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
