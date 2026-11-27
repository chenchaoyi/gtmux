package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageAgySession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	agyHome := filepath.Join(home, "agy-data")
	t.Setenv("AGY_HOME", agyHome)

	const sid = "agy-usage-session-1"
	dir := filepath.Join(agyHome, "brain", sid, ".system_generated", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	log := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-27T02:50:00Z","content":"hello"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T02:50:10Z","model":"gemini-4-neon-high","input_tokens":24000,"cache_read_tokens":0,"output_tokens":300}
{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T02:51:00Z","model":"gemini-4-neon-high","input_tokens":6000,"cache_read_tokens":25000,"output_tokens":500}
`
	if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}

	now, _ := time.Parse(time.RFC3339, "2026-11-27T02:52:00Z")
	snap, ok := ForSession("agy", sid, now)
	if !ok {
		t.Fatal("ForSession(agy) returned ok=false")
	}
	if snap.InTok != 30000 {
		t.Errorf("InTok = %d, want 30000", snap.InTok)
	}
	if snap.OutTok != 800 {
		t.Errorf("OutTok = %d, want 800", snap.OutTok)
	}
	if snap.CtxTok != 31000 {
		t.Errorf("CtxTok = %d, want 31000 (6000 + 25000)", snap.CtxTok)
	}
	if snap.Window <= 0 || snap.CtxFrac <= 0 {
		t.Errorf("Window=%d CtxFrac=%f, want positive", snap.Window, snap.CtxFrac)
	}
}
