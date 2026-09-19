package mine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The ledger bounds itself: a signature quiet for 90 days goes, a read mark whose log the
// agent deleted goes, and the pass history keeps about a year.
func TestTheLedgerBoundsItself(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	now := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	live := filepath.Join(dir, "live.jsonl")
	if err := os.WriteFile(live, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &ledger{
		Sources: map[string]sourceMark{live: {Offset: 3}, filepath.Join(dir, "deleted.jsonl"): {Offset: 9}},
		Emitted: map[string]bool{},
		Errors: map[string]*errorTally{
			"recent": {Line: "x", Last: now.Add(-24 * time.Hour).Unix()},
			"quiet":  {Line: "y", Last: now.Add(-91 * 24 * time.Hour).Unix()},
		},
	}
	sigs, srcs := l.prune(now)
	if sigs != 1 || srcs != 1 {
		t.Fatalf("pruned %d signatures and %d sources, want 1 and 1", sigs, srcs)
	}
	if l.Errors["recent"] == nil || l.Errors["quiet"] != nil {
		t.Errorf("signatures left: %v", l.Errors)
	}
	if _, ok := l.Sources[live]; !ok || len(l.Sources) != 1 {
		t.Errorf("sources left: %v", l.Sources)
	}

	var passes bytes.Buffer
	for i := 0; i < passesMaxLines+10; i++ {
		fmt.Fprintf(&passes, "{\"at\":%d}\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "passes.jsonl"), passes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.save(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "passes.jsonl"))
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	if len(lines) != passesKeep || string(lines[len(lines)-1]) != fmt.Sprintf("{\"at\":%d}", passesMaxLines+9) {
		t.Fatalf("passes.jsonl kept %d lines ending %s, want the newest %d", len(lines), lines[len(lines)-1], passesKeep)
	}
	if st := ReadStatus(dir, 5); st.LastPass != int64(passesMaxLines+9) {
		t.Errorf("the status view lost the last pass: %d", st.LastPass)
	}
}
