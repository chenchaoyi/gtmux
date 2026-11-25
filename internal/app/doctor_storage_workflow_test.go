package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
