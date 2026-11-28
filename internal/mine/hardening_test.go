package mine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCHH008_MineRunLockLastPassMarkerAndBoundedEmitted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "mine-ledger")
	logDir := t.TempDir()
	now := time.Unix(1_700_000_000, 0)

	// Seed >maxEmittedIDs correction IDs plus one active recurring-error ID.
	var preEmitted []string
	for i := 0; i < maxEmittedIDs+200; i++ {
		preEmitted = append(preEmitted, fmt.Sprintf("old-id-%05d", i))
	}
	preEmitted = append(preEmitted, "active-err-sig")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bEmit, _ := json.Marshal(preEmitted)
	if err := os.WriteFile(filepath.Join(dir, "emitted.json"), bEmit, 0o600); err != nil {
		t.Fatal(err)
	}
	bErr, _ := json.Marshal(map[string]*errorTally{
		"active-err-sig": {
			Line:      "permission denied",
			Count:     3,
			Sessions:  map[string]bool{"s1": true, "s2": true},
			First:     now.Unix() - 100,
			Last:      now.Unix() - 10,
			EmittedAt: now.Unix() - 5,
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "errors.json"), bErr, 0o600); err != nil {
		t.Fatal(err)
	}

	// Run two concurrent passes against the same ledger directory.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(offset int64) {
			defer wg.Done()
			_, err := Run(dir, Options{
				Roots: []Root{{Agent: "claude", Dir: logDir, Glob: "*.jsonl"}},
				Now:   now.Add(time.Duration(offset) * time.Second),
			})
			errs <- err
		}(int64(i))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Run error: %v", err)
		}
	}

	// Verify emitted.json was capped at maxEmittedIDs while preserving active-err-sig.
	var savedIDs []string
	rawSaved, err := os.ReadFile(filepath.Join(dir, "emitted.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawSaved, &savedIDs); err != nil {
		t.Fatalf("unmarshal emitted.json: %v", err)
	}
	if len(savedIDs) != maxEmittedIDs {
		t.Fatalf("len(emitted.json) = %d, want %d", len(savedIDs), maxEmittedIDs)
	}
	foundActiveErr := false
	for _, id := range savedIDs {
		if id == "active-err-sig" {
			foundActiveErr = true
			break
		}
	}
	if !foundActiveErr {
		t.Fatal("active recurring-error signature was evicted from bounded emitted.json")
	}

	// Verify LastPassAt reads last-pass marker without touching passes.jsonl / sources.json.
	if err := os.Chmod(filepath.Join(dir, "passes.jsonl"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "passes.jsonl"), 0o600) })
	if got := LastPassAt(dir); got < now.Unix() {
		t.Fatalf("LastPassAt = %d, want >= %d from fast last-pass marker", got, now.Unix())
	}
}
