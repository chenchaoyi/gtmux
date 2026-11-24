package hq

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS008HousekeepingFollowsGroupedHQFeed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	retired := state.StorePath("hq-feed", "spool.jsonl")
	live := state.StorePath("hq-feed", "last-self-check")
	original := filepath.Join(state.Dir(), "hq-feed", "spool.jsonl")
	for _, path := range []string{retired, live} {
		if err := state.WritePrivateFile(path, []byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}
	if got := RetiredPresent(); len(got) != 1 || got[0] != filepath.Join("hq-feed", "spool.jsonl") {
		t.Fatalf("grouped retired inventory: %v", got)
	}
	if err := state.WritePrivateFile(original, []byte("legacy conflict")); err != nil {
		t.Fatal(err)
	}
	Housekeep()
	if _, err := os.Stat(retired); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("grouped retired file remains: %v", err)
	}
	for _, path := range []string{live, original} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("housekeeping removed an unrelated file: %s: %v", path, err)
		}
	}
	if got := RetiredPresent(); len(got) != 0 {
		t.Fatalf("inactive path leaked into retired inventory: %v", got)
	}
}
