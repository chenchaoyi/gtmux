package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeAgyBackfillLog(t *testing.T, agyHome, sid, model string, ctx int) {
	t.Helper()
	dir := filepath.Join(agyHome, "brain", sid, ".system_generated", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T02:50:10Z","model":%q,"input_tokens":%d,"cache_read_tokens":0,"output_tokens":10}`, model, ctx)
	if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillLearnsAgyWindowsOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	agyHome := filepath.Join(home, "agy-data")
	t.Setenv("AGY_HOME", agyHome)

	writeAgyBackfillLog(t, agyHome, "old", "gemini-big", 900_000)
	if got := windowFor("agy", "gemini-big", 150_000, 0); got != 200_000 {
		t.Fatalf("precondition: windowFor = %d, want the 200k tier guess", got)
	}

	BackfillWindows()
	if got := windowFor("agy", "gemini-big", 150_000, 0); got != 1_000_000 {
		t.Fatalf("after backfill windowFor(agy) = %d, want 1,000,000", got)
	}
	if got := evidencedWindow("claude", "gemini-big"); got != 0 {
		t.Fatalf("agy evidence leaked into another agent: %d", got)
	}

	writeAgyBackfillLog(t, agyHome, "newer", "gemini-other", 800_000)
	BackfillWindows()
	if got := evidencedWindow("agy", "gemini-other"); got != 0 {
		t.Fatalf("backfill rescanned a log written after it ran: %d", got)
	}
}
