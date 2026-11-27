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

func TestAgySessionTitlePbtxtEscapesAndSafety(t *testing.T) {
	agyHome := t.TempDir()
	t.Setenv("AGY_HOME", agyHome)
	annDir := filepath.Join(agyHome, "annotations")
	if err := os.MkdirAll(annDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(annDir, "conv-1.pbtxt"), []byte(`title:"Quote \"A\" and hex \x42"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := AgySessionTitle("conv-1"); got != `Quote "A" and hex B` {
		t.Fatalf("AgySessionTitle(conv-1) = %q", got)
	}
	for _, bad := range []string{"", ".", "..", "../conv-1", "a/b", `a\b`} {
		if got := AgySessionTitle(bad); got != "" {
			t.Errorf("AgySessionTitle(%q) = %q, want empty", bad, got)
		}
	}
}

func TestAgySystemMessageFilteringAndToolActions(t *testing.T) {
	const sid = "sys-filter-sess"
	logContent := strings.Join([]string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-27T04:00:00Z","content":"The following is a <SYSTEM_MESSAGE> not actually sent by the user.\n\n<SYSTEM_MESSAGE>\nTask finished\n</SYSTEM_MESSAGE>"}`,
		`{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-11-27T04:01:00Z","content":"Inspect search tools\n\nThe following is a <SYSTEM_MESSAGE> not actually sent by the user.\n<SYSTEM_MESSAGE>Reminder: work efficiently.</SYSTEM_MESSAGE>"}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-11-27T04:01:05Z","model":"gemini-4-carbon-medium","tool_calls":[{"id":"tc1","name":"search_web","args":{"query":"\"openspec 2.2\""}},{"id":"tc2","name":"grep_search","args":{"Query":"SplitInputRegion","SearchPath":"/repo/internal"}}],"content":"Checked both."}`,
	}, "\n") + "\n"
	installAgyFixture(t, sid, logContent)

	turns, err := Load("agy", sid, 10)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("standalone <SYSTEM_MESSAGE> must not open a turn; got %d turns: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "Inspect search tools" {
		t.Fatalf("Prompt = %q, want %q", turns[0].Prompt, "Inspect search tools")
	}
	steps := turns[0].Segments[0].Steps
	if len(steps) != 2 {
		t.Fatalf("steps = %+v, want 2 tool steps", steps)
	}
	if steps[0].Operation != "web" || steps[0].Action != "web_search" || steps[0].Detail != "openspec 2.2" {
		t.Fatalf("search_web step = %#v, want Operation=web Action=web_search Detail='openspec 2.2'", steps[0])
	}
	if steps[1].Operation != "search" || steps[1].Title != "Grep" || steps[1].Detail != "SplitInputRegion" {
		t.Fatalf("grep_search step = %#v, want Operation=search Title=Grep Detail=SplitInputRegion", steps[1])
	}
}
