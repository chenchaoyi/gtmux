package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSW003DoctorStorageMaintenanceWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writersStopped := func() error { return nil }
	if err := state.ChangeStorageLayout(true, writersStopped); err != nil {
		t.Fatal(err)
	}
	if got := rowWorkflowStorageLayout(); got.value != "layout-v2" {
		t.Fatalf("initial layout: %+v", got)
	}
	confirmed := false
	confirm := func(message string) bool {
		confirmed = true
		if !strings.Contains(message, "tmux") {
			t.Fatalf("confirmation omitted the pre-restore window: %q", message)
		}
		return true
	}
	if code := applyDoctorStorageChange(true, true, func() error { return errors.New("tmux PID 47") }, confirm); code != 1 || confirmed {
		t.Fatalf("live writer admitted migration: code=%d confirm=%v", code, confirmed)
	}
	checks := 0
	interrupted := func() error {
		checks++
		if checks == 3 {
			return errors.New("simulated process appearing after publication")
		}
		return nil
	}
	if code := applyDoctorStorageChange(true, true, interrupted, confirm); code != 1 || !confirmed {
		t.Fatalf("interrupted migration: code=%d confirm=%v", code, confirmed)
	}
	output := captureStdout(t, func() {
		if rc := Run([]string{"--lang=en", "doctor", "--no-prompt"}); rc != 1 {
			t.Fatalf("pending doctor exit %d", rc)
		}
	})
	if !strings.Contains(output, "finish grouping") || !strings.Contains(output, "doctor --fix") {
		t.Fatalf("pending check omitted recovery %q", output)
	}
	if code := applyDoctorStorageChange(true, true, writersStopped, confirm); code != 0 {
		t.Fatalf("continue storage: %d", code)
	}
	if got := rowWorkflowStorageLayout(); got.status != stOK || got.value != "layout-v3" {
		t.Fatalf("readback after grouping: %+v", got)
	}
}

func TestGSW004DoctorCacheCleanupProtectsDeliveryState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	expired := filepath.Join(state.CacheDir(), ".private-"+strings.Repeat("A", 26))
	fresh := filepath.Join(state.CacheDir(), ".private-"+strings.Repeat("B", 26))
	ordinary := filepath.Join(state.CacheDir(), "icon.png")
	unknown := state.WorkflowPath("hq-nudges", ".private-"+strings.Repeat("C", 26))
	upload := filepath.Join(state.Dir(), "uploads", "referenced.png")
	for _, path := range []string{expired, fresh, ordinary, unknown, upload} {
		if err := state.WritePrivateFile(path, []byte("preserve independent records")); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{expired, ordinary, unknown, upload} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if got := rowCacheStaging(); got.status != stRec {
		t.Fatalf("cache inspection did not describe the candidate: %+v", got)
	}
	s := fixState{yes: true}
	if got := s.stepCacheStaging(); got != 1 || s.rc != 0 {
		t.Fatalf("removed=%d rc=%d", got, s.rc)
	}
	if _, err := os.Stat(expired); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired stage remains: %v", err)
	}
	for _, path := range []string{fresh, ordinary, unknown, upload} {
		if got, err := os.ReadFile(path); err != nil || string(got) != "preserve independent records" {
			t.Fatalf("cleanup changed %s: %q %v", path, got, err)
		}
	}
	if got := s.stepCacheStaging(); got != 0 {
		t.Fatalf("repeated cleanup removed %d files", got)
	}
}

func TestDWI002DoctorWakeStorageShowsIndependentDiagnostics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	originalLanguage := i18n.Lang()
	defer i18n.SetLang(originalLanguage)
	writeWake := func(name string) {
		t.Helper()
		if err := state.WritePrivateFile(state.WorkflowPath("hq-nudges", name), []byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}
	writeWake("10.p1.txt.refused")
	writeWake("20.p1.hqrelay.refused")
	i18n.SetLang("en")
	if row := rowWakeStorage(); row.status != stInfo ||
		!strings.Contains(row.value, "2 refused and retained") ||
		!strings.Contains(row.value, "0 unrecognized") ||
		strings.Contains(row.note, "unrecognized") || strings.Contains(row.note, "unclassified") {
		t.Fatalf("refusals alone should be informational: %+v", row)
	}

	for _, name := range []string{"30.p1.txt", "40.p1.txt.sending", "40.p1.txt.native-uncertain", "foreign-file"} {
		writeWake(name)
	}
	for _, tc := range []struct {
		lang, value string
		notes       []string
	}{
		{"en", "1 queued · 1 claimed · 2 refused and retained · 1 unknown · 0 awaiting proof · 1 unrecognized",
			[]string{"Check delivery evidence for each batch", "Distinct claimed and retained files share 1 wake slot", "Verify ownership and delivery evidence for unrecognized entries"}},
		{"zh", "1 待投递 · 1 已领取 · 2 已拒绝留存 · 1 结果未知 · 0 待送达核实 · 1 未识别",
			[]string{"按批次核对送达证据", "1 个投递槽位同时存在不同的领取文件和留存文件", "请核对未识别条目的归属和投递证据"}},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			i18n.SetLang(tc.lang)
			row := rowWakeStorage()
			if row.status != stRec || row.value != tc.value {
				t.Fatalf("unexpected wake row: %+v", row)
			}
			for _, note := range tc.notes {
				if !strings.Contains(row.note, note) {
					t.Errorf("missing %q in %q", note, row.note)
				}
			}
		})
	}
}
