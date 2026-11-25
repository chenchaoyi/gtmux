package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDoctorPreparedStorageDiagnosticThroughCLI(t *testing.T) {
	const (
		normalHint   = "continue or cancel the prepared move"
		conflictText = "the prepared service fence does not match the base layout"
	)
	for _, tc := range []struct {
		name     string
		conflict bool
		want     string
		reject   string
	}{
		{name: "valid preparation", want: normalHint, reject: conflictText},
		{name: "conflicting marker", conflict: true, want: conflictText, reject: normalHint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("PATH", t.TempDir())
			if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			if err := state.PrepareWorkflowServiceFence(); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(state.Dir(), ".storage-layout-v3")
			var originalMarker os.FileInfo
			if tc.conflict {
				if err := os.Mkdir(marker, 0700); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(marker)
				if err != nil {
					t.Fatal(err)
				}
				originalMarker = info
			}
			status, validation := state.ReadStorageLayout()
			if !status.ServicePreparation() || (validation != nil) != tc.conflict {
				t.Fatalf("unexpected prepared fixture: %+v, %v", status, validation)
			}
			journal := filepath.Join(state.Dir(), ".storage-layout-transition.json")
			original, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			output := runStorageDoctorPTY(t, "", func() {
				if rc := Run([]string{"--lang=en", "doctor", "--fix"}); rc != 1 {
					t.Fatalf("doctor --fix exit = %d, want 1", rc)
				}
			})
			if !strings.Contains(output, tc.want) || strings.Contains(output, tc.reject) {
				t.Errorf("doctor --fix output = %q, want %q without %q", output, tc.want, tc.reject)
			}
			if lease, err := state.EnterStorageCommand(); err == nil {
				lease.Close()
				t.Error("ordinary storage remained available during prepared maintenance")
			}
			if got, err := os.ReadFile(journal); err != nil || !bytes.Equal(got, original) {
				t.Errorf("prepared journal changed: %v", err)
			}
			if info, err := os.Stat(marker); tc.conflict {
				if err != nil || !os.SameFile(info, originalMarker) {
					t.Errorf("conflicting marker changed: %v", err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("unexpected workflow marker after Doctor: %v", err)
			}
		})
	}
}
