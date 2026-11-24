package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS011DoctorAliasSharesLayoutAdmission(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	data := state.StorePath("events.seq")
	if err := state.WritePrivateFile(data, []byte("11")); err != nil {
		t.Fatal(err)
	}
	paused := errors.New("stop after journaling")
	calls := 0
	if err := state.ChangeStorageLayout(true, func() error {
		calls++
		if calls == 2 {
			return paused
		}
		return nil
	}); !errors.Is(err, paused) {
		t.Fatalf("prepare transition: %v", err)
	}
	output := captureStdout(t, func() {
		if code := Run([]string{"--lang=en", "dr", "--storage-layout"}); code != 0 {
			t.Fatalf("doctor alias is gated during migration: %d", code)
		}
	})
	if !strings.Contains(output, "Transition pending") {
		t.Fatalf("alias did not inspect the transition: %q", output)
	}
	for _, flag := range []string{"--group-storage", "--flatten-storage"} {
		if lease, ok := enterCommandLayout("dr", []string{flag}); !ok || lease != nil {
			lease.Close()
			t.Fatalf("alias %s would contend with its own transition: %v", flag, ok)
		}
	}
}
