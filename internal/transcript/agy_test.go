package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func installAgyFixture(t *testing.T, sessionID, content string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("AGY_HOME", home)
	dir := filepath.Join(home, "brain", sessionID, ".system_generated", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgyTranscriptLoadAndMetadata(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "agy-current.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	const sid = "eb857b74-63c6-42fd-8469-cc01c3053a94"
	installAgyFixture(t, sid, string(raw))

	turns, err := Load("agy", sid, 10)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2: %+v", len(turns), turns)
	}

	// Turn 1: <USER_REQUEST> unwrapped, metadata stripped, errored draft skipped, two tools recorded.
	if turns[0].Prompt != "Fix the failing unit test" {
		t.Errorf("turn[0].Prompt = %q", turns[0].Prompt)
	}
	if turns[0].Response != "Fixed the failing test and verified all packages pass." {
		t.Errorf("turn[0].Response = %q", turns[0].Response)
	}
	if strings.Contains(turns[0].Response, "Partial draft") {
		t.Fatal("errored PLANNER_RESPONSE partial draft leaked into response")
	}
	var ops []string
	for _, seg := range turns[0].Segments {
		for _, st := range seg.Steps {
			ops = append(ops, st.Operation+":"+st.Title)
		}
	}
	if got := strings.Join(ops, ","); got != "command:Shell,edit:Edit" {
		t.Errorf("turn[0] steps = %q, want command:Shell,edit:Edit", got)
	}

	// Turn 2: plain prompt + search_web mapped to web operation.
	if turns[1].Prompt != "Check web docs" || turns[1].Response != "Docs verified." {
		t.Errorf("turn[1] = %+v", turns[1])
	}
	if len(turns[1].Segments) == 0 || len(turns[1].Segments[0].Steps) != 1 || turns[1].Segments[0].Steps[0].Operation != "web" {
		t.Errorf("turn[1] web step = %+v", turns[1].Segments)
	}

	// Timestamps from RFC3339 created_at.
	wantFirst, _ := time.Parse(time.RFC3339, "2026-11-27T02:50:08Z")
	wantLast, _ := time.Parse(time.RFC3339, "2026-11-27T02:51:05Z")
	if got := FirstMessageTime("Antigravity", sid); got != wantFirst.Unix() {
		t.Errorf("FirstMessageTime = %d, want %d", got, wantFirst.Unix())
	}
	if got := LastMessageTime("agy", sid); got != wantLast.Unix() {
		t.Errorf("LastMessageTime = %d, want %d", got, wantLast.Unix())
	}
	if errored, msg := LastMessageError("agy", sid); errored || msg != "" {
		t.Errorf("normal end flagged error: (%v, %q)", errored, msg)
	}
	if got := AgentForSession(sid); got != "agy" {
		t.Errorf("AgentForSession = %q, want agy", got)
	}
	if len(LogGlobs()["agy"]) == 0 {
		t.Fatal("LogGlobs missing agy pattern")
	}
}

func TestAgyTranscriptErrorAndPathTraversalGuard(t *testing.T) {
	installAgyFixture(t, "err-sess", `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-27T03:00:00Z","content":"Run build"}
{"step_index":1,"source":"SYSTEM","type":"ERROR_MESSAGE","status":"DONE","created_at":"2026-11-27T03:00:02Z","error":"API error: rate limit exceeded"}
`)
	errored, summary := LastMessageError("agy", "err-sess")
	if !errored || !strings.Contains(summary, "rate limit exceeded") {
		t.Fatalf("LastMessageError = (%v, %q), want true with rate limit summary", errored, summary)
	}

	for _, bad := range []string{"", ".", "..", "../escape", "a/b", `a\b`} {
		if p := LogPath("agy", bad); p != "" {
			t.Errorf("LogPath(agy, %q) = %q, want empty", bad, p)
		}
	}
}
