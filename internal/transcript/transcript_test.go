package transcript

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// allSteps flattens every tool step across a turn's segments (steps now live per
// segment rather than in one flat list).
func allSteps(t Turn) []Step {
	var out []Step
	for _, s := range t.Segments {
		out = append(out, s.Steps...)
	}
	return out
}

// segTexts collects a turn's non-empty segment texts in order.
func segTexts(t Turn) []string {
	var out []string
	for _, s := range t.Segments {
		if s.Text != "" {
			out = append(out, s.Text)
		}
	}
	return out
}

// writeLog lays out a session log under a fake HOME so the glob-based loaders
// find it, and returns nothing (the test reads via Load).
func writeClaudeLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "-some-project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, sessionID+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCodexLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "06", "28")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	name := "rollout-2026-06-28T10-00-00-" + sessionID + ".jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCodexLastTurnBoundary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	start := `{"timestamp":"2026-09-27T10:14:00Z","type":"event_msg","payload":{"type":"task_started"}}`
	complete := `{"timestamp":"2026-09-27T10:15:58.590Z","type":"event_msg","payload":{"type":"task_complete"}}`
	writeCodexLog(t, home, "done", []string{start, complete, `{"timestamp":"2026-09-27T10:16:00Z","type":"event_msg","payload":{"type":"token_count"}}`})
	if kind, at := CodexLastTurnBoundary("done"); kind != "task_complete" || at.Format(time.RFC3339Nano) != "2026-09-27T10:15:58.59Z" {
		t.Fatalf("last boundary = %q %s", kind, at)
	}
	writeCodexLog(t, home, "new-turn", []string{start, complete, `{"timestamp":"2026-09-27T10:17:00Z","type":"event_msg","payload":{"type":"task_started"}}`})
	if kind, _ := CodexLastTurnBoundary("new-turn"); kind != "task_started" {
		t.Fatalf("new turn boundary = %q", kind)
	}
}

func TestCodexPathScopeDiscoversOnceAndKeepsFactsFresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "10", "15")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	const sid = "scoped"
	path := filepath.Join(dir, "rollout-2026-10-15T10-00-00-"+sid+".jsonl")
	line := `{"type":"session_meta","payload":{"id":"` + sid + `","originator":"codex-tui"}}` + "\n" +
		`{"timestamp":"2026-10-15T10:00:00Z","type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	count := 0
	scope := newCodexPathScope(func() []string { count++; return enumerateCodexCandidates() })
	for _, invalid := range []string{"", "bad*", "bad/", "bad?", "bad[", "bad]", `bad\\`} {
		if got := scope.Paths(invalid); got != nil || count != 0 {
			t.Fatalf("invalid lookup %q = %v, discovery count %d", invalid, got, count)
		}
	}
	paths := scope.Paths(sid)
	if len(paths) != 1 || count != 1 {
		t.Fatalf("first lookup = %v, discovery count %d", paths, count)
	}
	paths[0] = "mutated"
	if got := scope.Paths(sid); len(got) != 1 || got[0] != path || count != 1 {
		t.Fatalf("memoized paths = %v, discovery count %d", got, count)
	}
	if kind, _ := scope.CodexLastTurnBoundary(sid); kind != "task_complete" {
		t.Fatalf("boundary = %q", kind)
	}
	if got := scope.LastMessageTime(sid); got != time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("last message time = %d", got)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(`{"timestamp":"2026-10-15T10:01:00Z","type":"event_msg","payload":{"type":"turn_aborted"}}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if kind, _ := scope.CodexLastTurnBoundary(sid); kind != "turn_aborted" {
		t.Fatalf("fresh boundary = %q", kind)
	}
	if got := scope.LastMessageTime(sid); got != time.Date(2026, 10, 15, 10, 1, 0, 0, time.UTC).Unix() {
		t.Fatalf("fresh last message time = %d", got)
	}
	if _, ok := scope.CodexIdentityFor(sid); !ok || count != 1 {
		t.Fatalf("identity scope lookup failed, discovery count %d", count)
	}
	continuation := filepath.Join(dir, "rollout-2026-10-15T10-02-00-"+sid+"_instance-2.jsonl")
	if err := os.WriteFile(continuation, []byte(
		`{"type":"session_meta","payload":{"id":"`+sid+`"}}`+"\n"+
			`{"timestamp":"2026-10-15T10:02:00Z","type":"event_msg","payload":{"type":"task_complete"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := scope.Paths(sid); len(got) != 1 {
		t.Fatalf("current scope changed its candidate snapshot: %v", got)
	}
	next := newCodexPathScope(func() []string { count++; return enumerateCodexCandidates() })
	if got := next.Paths(sid); len(got) != 2 || count != 2 {
		t.Fatalf("new scope discovery count %d", count)
	}
	if got := next.LastMessageTime(sid); got != time.Date(2026, 10, 15, 10, 2, 0, 0, time.UTC).Unix() {
		t.Fatalf("new scope missed a new continuation: %d", got)
	}
}

func TestCodexPathScopeMatchesDirectCompatibilityMatrix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	liveDirs := []string{
		filepath.Join(home, "sessions", "2026", "10", "14"),
		filepath.Join(home, "sessions", "2026", "10", "15"),
		filepath.Join(home, "archived_sessions"),
	}
	for _, dir := range liveDirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, name, meta string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(meta+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	meta := func(field, id string) string {
		return `{"type":"session_meta","payload":{"` + field + `":"` + id + `"}}`
	}
	const sid = "matrix"
	base := "rollout-2026-10-14T10-00-00-" + sid + ".jsonl"
	continuation := "rollout-2026-10-15T10-00-00-" + sid + "_instance-2.jsonl"
	write(liveDirs[0], base, meta("id", sid))
	write(liveDirs[1], continuation, meta("id", sid))
	write(liveDirs[1], "rollout-2026-10-15T09-00-00-foreign-base.jsonl", meta("id", "other"))
	write(liveDirs[1], "rollout-2026-10-15T11-00-00-"+sid+"_foreign.jsonl", meta("id", "other"))
	write(liveDirs[1], "rollout-2026-10-15T12-00-00-"+sid+"_idless.jsonl", `{"type":"session_meta","payload":{}}`)
	write(liveDirs[2], base, meta("id", sid)) // live copy must win on duplicate basename
	archiveOnly := "rollout-2026-10-15T13-00-00-" + sid + ".jsonl"
	write(liveDirs[2], archiveOnly, meta("id", sid))
	write(liveDirs[1], "rollout-2026-10-15T14-00-00-legacy.jsonl", meta("session_id", "legacy"))
	idlessBase := "rollout-2026-10-15T15-00-00-idless.jsonl"
	write(liveDirs[1], idlessBase, `{"type":"session_meta","payload":{}}`)

	scope := NewCodexPathScope()
	for _, id := range []string{sid, "legacy", "idless", "unknown"} {
		direct := codexLogPathsFresh(id)
		scoped := scope.Paths(id)
		if !reflect.DeepEqual(scoped, direct) {
			t.Errorf("%s scoped paths = %v, direct = %v", id, scoped, direct)
		}
	}
	want := []string{base, continuation, archiveOnly}
	got := scope.Paths(sid)
	gotNames := make([]string, len(got))
	for i, path := range got {
		gotNames[i] = filepath.Base(path)
	}
	if !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("matrix paths = %v, want ordered %v", gotNames, want)
	}
	if got := filepath.Dir(scope.Paths(sid)[0]); got != liveDirs[0] {
		t.Fatalf("duplicate basename resolved to %q, want live directory %q", got, liveDirs[0])
	}
	if paths := scope.Paths("legacy"); len(paths) != 1 {
		t.Fatalf("legacy session_id paths = %v", paths)
	}
	if paths := scope.Paths("idless"); len(paths) != 1 || filepath.Base(paths[0]) != idlessBase {
		t.Fatalf("idless base paths = %v", paths)
	}
	otherHome := t.TempDir()
	t.Setenv("CODEX_HOME", otherHome)
	otherDir := filepath.Join(otherHome, "sessions", "2026", "10", "15")
	if err := os.MkdirAll(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(otherDir, "rollout-2026-10-15T10-00-00-"+sid+".jsonl")
	if err := os.WriteFile(otherPath, []byte(meta("id", sid)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	otherScope := NewCodexPathScope()
	if got := otherScope.Paths(sid); len(got) != 1 || got[0] != otherPath {
		t.Fatalf("new CODEX_HOME scope paths = %v, want %q", got, otherPath)
	}
}

func TestCodexPathScopeConcurrentReadsAndIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "10", "15")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-10-15T10-00-00-concurrent.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"concurrent"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var discoveries [2]atomic.Int32
	scopes := [2]*CodexPathScope{
		newCodexPathScope(func() []string { discoveries[0].Add(1); return enumerateCodexCandidates() }),
		newCodexPathScope(func() []string { discoveries[1].Add(1); return enumerateCodexCandidates() }),
	}
	var wg sync.WaitGroup
	for _, scope := range scopes {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(scope *CodexPathScope) {
				defer wg.Done()
				for n := 0; n < 10; n++ {
					if got := scope.Paths("concurrent"); len(got) != 1 || got[0] != path {
						t.Errorf("concurrent paths = %v", got)
					}
					if _, ok := scope.CodexIdentityFor("concurrent"); !ok {
						t.Error("concurrent identity lookup failed")
					}
				}
			}(scope)
		}
	}
	wg.Wait()
	for i, scope := range scopes {
		if got := discoveries[i].Load(); got != 1 || scope.DiscoveryCount() != 1 {
			t.Errorf("scope %d discoveries = %d (counter %d), want one", i, got, scope.DiscoveryCount())
		}
	}
}

func TestCodexResumedRolloutsBelongToOneSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "09", "29")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	const sid = "session-1"
	write := func(name, id string, lines ...string) {
		t.Helper()
		body := `{"type":"session_meta","payload":{"id":"` + id + `","originator":"codex_work_desktop"}}` + "\n" + strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := "rollout-2026-09-29T07-00-00-" + sid + ".jsonl"
	newer := "rollout-2026-09-29T07-10-00-" + sid + "_instance-2.jsonl"
	write(old, sid,
		`{"timestamp":"2026-09-29T07:00:00Z","type":"event_msg","payload":{"type":"task_started"}}`,
		`{"timestamp":"2026-09-29T07:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"first question"}}`,
		`{"timestamp":"2026-09-29T07:00:02Z","type":"event_msg","payload":{"type":"turn_aborted"}}`)
	write(newer, sid,
		`{"timestamp":"2026-09-29T07:10:00Z","type":"event_msg","payload":{"type":"task_started"}}`,
		`{"timestamp":"2026-09-29T07:10:01Z","type":"event_msg","payload":{"type":"user_message","message":"second question"}}`,
		`{"timestamp":"2026-09-29T07:10:02Z","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"second answer"}}`)
	// The filename looks like a continuation but its own metadata says otherwise.
	write("rollout-2026-09-29T07-20-00-"+sid+"_other.jsonl", "different-session",
		`{"timestamp":"2026-09-29T07:20:00Z","type":"event_msg","payload":{"type":"task_started"}}`)

	if paths := codexLogPaths(sid); len(paths) != 2 {
		t.Fatalf("matching rollout paths = %v", paths)
	}
	if got := filepath.Base(LogPath("codex", sid)); got != old {
		t.Fatalf("raw log path = %q, want original file for byte-offset readers", got)
	}
	revision := LogRevision("codex", sid)
	if revision == "" {
		t.Fatal("missing multi-rollout chat revision")
	}
	if kind, at := CodexLastTurnBoundary(sid); kind != "task_complete" || at.Format(time.RFC3339) != "2026-09-29T07:10:02Z" {
		t.Fatalf("latest boundary = %q %s", kind, at)
	}
	if got := FirstMessageTime("codex", sid); got != time.Date(2026, 9, 29, 7, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("first time = %d", got)
	}
	if got := LastMessageTime("codex", sid); got != time.Date(2026, 9, 29, 7, 10, 2, 0, time.UTC).Unix() {
		t.Errorf("last time = %d", got)
	}
	scope := NewCodexPathScope()
	if got, want := scope.Paths(sid), codexLogPathsFresh(sid); !reflect.DeepEqual(got, want) {
		t.Errorf("scoped paths = %v, direct = %v", got, want)
	}
	if kind, at := scope.CodexLastTurnBoundary(sid); kind != "task_complete" || at.Format(time.RFC3339) != "2026-09-29T07:10:02Z" {
		t.Errorf("scoped latest boundary = %q %s", kind, at)
	}
	if got, want := scope.LastMessageTime(sid), LastMessageTime("codex", sid); got != want {
		t.Errorf("scoped last message = %d, direct = %d", got, want)
	}
	if got, ok := scope.CodexIdentityFor(sid); !ok || got.CreationClient != "chatgpt_desktop" {
		t.Errorf("scoped identity = %+v, %v", got, ok)
	}
	turns, err := Load("codex", sid, 10)
	if err != nil || len(turns) != 2 || turns[0].Prompt != "first question" || turns[1].Prompt != "second question" || turns[1].Response != "second answer" {
		t.Fatalf("combined turns = %+v, err %v", turns, err)
	}
	if recent, err := Load("codex", sid, 1); err != nil || len(recent) != 1 || recent[0].Prompt != "second question" {
		t.Fatalf("recent continuation turn = %+v, err %v", recent, err)
	}
	// A new turn in the newest file must override its prior completion.
	f, err := os.OpenFile(filepath.Join(dir, newer), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(`{"timestamp":"2026-09-29T07:11:00Z","type":"event_msg","payload":{"type":"task_started"}}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if kind, _ := CodexLastTurnBoundary(sid); kind != "task_started" {
		t.Fatalf("newest turn boundary = %q", kind)
	}
	if next := LogRevision("codex", sid); next == revision {
		t.Fatal("chat revision did not change when the resumed rollout grew")
	}
}

// CodexSessionForCwd derives a Codex pane's session id only if its cwd identifies
// one rollout. Recency cannot distinguish parallel sessions in the same repo.
func TestCodexSessionForCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	meta := func(sid, cwd string) []string {
		return []string{`{"timestamp":"t","type":"session_meta","payload":{"type":"session_meta","session_id":"` + sid + `","cwd":"` + cwd + `"}}`}
	}
	currentMeta := func(sid, cwd string) []string {
		return []string{`{"timestamp":"t","type":"session_meta","payload":{"id":"` + sid + `","cwd":"` + cwd + `","originator":"codex-tui"}}`}
	}
	writeCodexLog(t, home, "old-sid", meta("old-sid", "/proj/a"))
	writeCodexLog(t, home, "other", meta("other", "/proj/b"))
	writeCodexLog(t, home, "current", currentMeta("current", "/proj/current"))
	if sid, ok := CodexSessionForCwd("/proj/current"); !ok || sid != "current" {
		t.Errorf("current session_meta.id → %q,%v; want current,true", sid, ok)
	}

	if sid, ok := CodexSessionForCwd("/proj/b"); !ok || sid != "other" {
		t.Errorf("cwd /proj/b → %q,%v; want other,true", sid, ok)
	}
	if _, ok := CodexSessionForCwd("/proj/none"); ok {
		t.Error("unknown cwd should not match")
	}
	if _, ok := CodexSessionForCwd(""); ok {
		t.Error("empty cwd should not match")
	}

	// A second rollout for the same cwd makes the result ambiguous, even when its
	// mtime is newer: recency is not a reliable session identity.
	newer := filepath.Join(home, ".codex", "sessions", "2026", "06", "28", "rollout-2026-06-28T10-00-00-new-sid.jsonl")
	os.WriteFile(newer, []byte(meta("new-sid", "/proj/a")[0]+"\n"), 0o644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(newer, future, future)
	if sid, ok := CodexSessionForCwd("/proj/a"); ok || sid != "" {
		t.Errorf("ambiguous cwd /proj/a → %q,%v; want empty,false", sid, ok)
	}
}

func TestLoadClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "11111111-2222-3333-4444-555555555555"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"ai-title","title":"ignored"}`,
		`{"type":"user","message":{"role":"user","content":"fix the build"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"text","text":"Let me look"},{"type":"tool_use","name":"Edit","input":{"file_path":"/x/main.go"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Done — fixed it."}]}}`,
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
		`{"type":"user","message":{"role":"user","content":"now run tests"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"All green."}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %d: %+v", len(turns), turns)
	}
	// both assistant text segments are kept (preamble + closing), not just the last.
	if turns[0].Prompt != "fix the build" || turns[0].Response != "Let me look\n\nDone — fixed it." {
		t.Fatalf("turn0 mismatch: %+v", turns[0])
	}
	if len(allSteps(turns[0])) != 1 || allSteps(turns[0])[0].Title != "Edit" || allSteps(turns[0])[0].Detail != "/x/main.go" {
		t.Fatalf("turn0 steps mismatch: %+v", allSteps(turns[0]))
	}
	if turns[1].Prompt != "now run tests" || turns[1].Response != "All green." {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
	if len(allSteps(turns[1])) != 1 || allSteps(turns[1])[0].Title != "Bash" {
		t.Fatalf("turn1 steps mismatch: %+v", allSteps(turns[1]))
	}
}

// The prompt's log timestamp is captured onto Turn.Time (chat-view time label).
func TestLoadClaudeCapturesPromptTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","timestamp":"2026-06-15T04:15:35.802Z","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","timestamp":"2026-06-15T04:15:40.000Z","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"hi"}]}}`,
	})
	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn, got %d", len(turns))
	}
	if turns[0].Time != "2026-06-15T04:15:35.802Z" {
		t.Fatalf("want prompt timestamp, got %q", turns[0].Time)
	}
}

// Codex turns carry the user_message line's timestamp too.
func TestLoadCodexCapturesPromptTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "ffffffff-1111-2222-3333-444444444444"
	writeCodexLog(t, home, sid, []string{
		`{"timestamp":"2026-06-28T10:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"build it"}}`,
		`{"timestamp":"2026-06-28T10:00:05.000Z","type":"event_msg","payload":{"type":"agent_message","message":"done"}}`,
	})
	turns, err := Load("Codex", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].Time != "2026-06-28T10:00:01.000Z" {
		t.Fatalf("codex time mismatch: %+v", turns)
	}
}

func TestLoadCodexShowsCommentaryBeforeTurnCompletes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "codex-commentary-before-final"
	writeCodexLog(t, home, sid, []string{
		`{"timestamp":"2026-09-29T10:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the mobile view"}]}}`,
		`{"timestamp":"2026-09-29T10:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"analysis","content":[{"type":"output_text","text":"private reasoning"}]}}`,
		`{"timestamp":"2026-09-29T10:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"I found the display path."}]}}`,
		`{"timestamp":"2026-09-29T10:00:03Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}}`,
		`{"timestamp":"2026-09-29T10:00:04Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"The targeted tests pass."}]}}`,
	})
	turns, err := Load("codex", sid, 10)
	if err != nil || len(turns) != 1 {
		t.Fatalf("Load before completion = %+v, %v", turns, err)
	}
	if got := segTexts(turns[0]); !reflect.DeepEqual(got, []string{"I found the display path.", "The targeted tests pass."}) {
		t.Fatalf("in-progress commentary = %q", got)
	}
	if len(turns[0].Segments[0].Steps) != 1 || turns[0].Segments[0].Steps[0].Title != "exec" {
		t.Fatalf("tool call did not stay between commentary bubbles: %+v", turns[0].Segments)
	}
	if strings.Contains(turns[0].Response, "private reasoning") {
		t.Fatalf("analysis leaked into chat: %q", turns[0].Response)
	}

	path := codexLogPath(sid)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(strings.Join([]string{
		`{"timestamp":"2026-09-29T10:00:05Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"Fixed it."}]}}`,
		`{"timestamp":"2026-09-29T10:00:05Z","type":"event_msg","payload":{"type":"agent_message","message":"Fixed it."}}`,
		`{"timestamp":"2026-09-29T10:00:06Z","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"Fixed it."}}`,
	}, "\n") + "\n")
	if closeErr := f.Close(); err != nil || closeErr != nil {
		t.Fatalf("append final answer: %v, close: %v", err, closeErr)
	}
	turns, err = Load("codex", sid, 10) // cached loader must pick up the growing log
	if err != nil || len(turns) != 1 {
		t.Fatalf("Load after completion = %+v, %v", turns, err)
	}
	if got := segTexts(turns[0]); !reflect.DeepEqual(got, []string{"I found the display path.", "The targeted tests pass.", "Fixed it."}) {
		t.Fatalf("completed reply duplicated or lost text: %q", got)
	}
}

// A turn's reply spread across several assistant messages (text → tool → text →
// tool → text) keeps EVERY text segment, not just the last one.
func TestLoadClaudeKeepsAllReplySegments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"do the thing"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"First, some context."},{"type":"tool_use","name":"Read","input":{"file_path":"/a"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Now the middle bit."},{"type":"tool_use","name":"Edit","input":{"file_path":"/a"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"ok"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"And the conclusion."}]}}`,
	})
	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := "First, some context.\n\nNow the middle bit.\n\nAnd the conclusion."
	if len(turns) != 1 || turns[0].Response != want {
		t.Fatalf("want all 3 segments %q, got %+v", want, turns)
	}
	wantSegs := []string{"First, some context.", "Now the middle bit.", "And the conclusion."}
	got := segTexts(turns[0])
	if len(got) != len(wantSegs) {
		t.Fatalf("want %d segment texts, got %v", len(wantSegs), got)
	}
	for i, s := range wantSegs {
		if got[i] != s {
			t.Fatalf("segment %d = %q, want %q", i, got[i], s)
		}
	}
	// the Read step attaches to its own segment (text "First…"), the Edit to the
	// second — i.e. steps interleave with the bubbles in order.
	if len(turns[0].Segments) != 3 || len(turns[0].Segments[0].Steps) != 1 || turns[0].Segments[0].Steps[0].Title != "Read" {
		t.Fatalf("segment/step interleave wrong: %+v", turns[0].Segments)
	}
	if len(turns[0].Segments[1].Steps) != 1 || turns[0].Segments[1].Steps[0].Title != "Edit" {
		t.Fatalf("segment 1 step wrong: %+v", turns[0].Segments[1])
	}
}

// Harness-injected user turns (<task-notification>, <system-reminder>) must not
// show up as chat prompts; a reminder appended to a real prompt is trimmed off.
func TestLoadClaudeSkipsHarnessBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "22222222-3333-4444-5555-666666666666"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"real question"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"an answer"}]}}`,
		// a pure background-task notice injected as a user turn — NOT a prompt
		`{"type":"user","message":{"role":"user","content":"<task-notification>\n<task-id>abc</task-id>\n<status>completed</status>\n</task-notification>"}}`,
		// a pure context reminder injected as a user turn — NOT a prompt
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"<system-reminder>be nice</system-reminder>"}]}}`,
		// a real prompt with a reminder appended by the harness — keep only the prompt
		`{"type":"user","message":{"role":"user","content":"second question\n<system-reminder>context here</system-reminder>"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"second answer"}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns (harness blocks dropped), got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "real question" {
		t.Fatalf("turn0 prompt mismatch: %q", turns[0].Prompt)
	}
	if turns[1].Prompt != "second question" || turns[1].Response != "second answer" {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
}

// Rejecting a tool with "No, and tell Claude what to do" embeds the typed message
// in the rejection tool_result — it must surface as a user turn (it used to vanish).
func TestLoadClaudeShowsRejectFeedback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "33333333-4444-5555-6666-777777777777"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"edit main.go"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/x/main.go"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). To tell you how to proceed, the user said:\nuse a map instead of a slice"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"Got it, switching to a map."}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns (reject feedback opens one), got %d: %+v", len(turns), turns)
	}
	if turns[1].Prompt != "use a map instead of a slice" {
		t.Fatalf("reject-feedback prompt mismatch: %q", turns[1].Prompt)
	}
	if turns[1].Response != "Got it, switching to a map." {
		t.Fatalf("turn1 response mismatch: %q", turns[1].Response)
	}
}

// A normal tool OUTPUT result and a plain rejection (no typed feedback) must NOT
// open a turn — only a rejection that carries the user's own message does.
func TestLoadClaudePlainRejectionNotATurn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "44444444-5555-6666-7777-888888888888"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"do it"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file1\nfile2"}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"The user doesn't want to proceed with this tool use. STOP what you are doing and wait for the user to tell you how to proceed."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn (output/plain-reject open none), got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "do it" {
		t.Fatalf("turn0 prompt mismatch: %q", turns[0].Prompt)
	}
}

func TestLoadClaudeMaxTurns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines,
			`{"type":"user","message":{"role":"user","content":"q"}}`,
			`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"a"}]}}`,
		)
	}
	writeClaudeLog(t, home, sid, lines)
	turns, _ := Load("claude", sid, 2)
	if len(turns) != 2 {
		t.Fatalf("want 2 (last-N), got %d", len(turns))
	}
}

// appendClaudeLog appends lines to an existing session log (simulating a growing
// live log between polls).
func appendClaudeLog(t *testing.T, home, sessionID string, lines []string) {
	t.Helper()
	p := filepath.Join(home, ".claude", "projects", "-some-project", sessionID+".jsonl")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

// A growing log must re-parse incrementally: the last cached turn picks up its
// appended content, and new turns are spliced on — without dropping or
// duplicating turns. This exercises Loader's growth path.
func TestLoadClaudeIncrementalGrowth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "abababab-cdcd-efef-0101-202020202020"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"q1"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a1"}]}}`,
		`{"type":"user","message":{"role":"user","content":"q2"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a2 partial"}]}}`,
	})

	turns, _ := Load("claude", sid, 50)
	if len(turns) != 2 || turns[1].Prompt != "q2" || turns[1].Response != "a2 partial" {
		t.Fatalf("pre-growth mismatch: %+v", turns)
	}

	// The open turn (q2) gains more output, then a brand-new turn (q3) arrives.
	appendClaudeLog(t, home, sid, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a2 final"}]}}`,
		`{"type":"user","message":{"role":"user","content":"q3"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"a3"}]}}`,
	})

	turns, _ = Load("claude", sid, 50)
	if len(turns) != 3 {
		t.Fatalf("want 3 turns after growth, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "q1" || turns[0].Response != "a1" {
		t.Fatalf("turn0 clobbered: %+v", turns[0])
	}
	// q2 accumulates both reply segments across the Bash step (text → tool → text).
	if turns[1].Prompt != "q2" || turns[1].Response != "a2 partial\n\na2 final" {
		t.Fatalf("turn1 not updated incrementally: %+v", turns[1])
	}
	if len(allSteps(turns[1])) != 1 || allSteps(turns[1])[0].Title != "Bash" {
		t.Fatalf("turn1 step not picked up: %+v", allSteps(turns[1]))
	}
	if turns[2].Prompt != "q3" || turns[2].Response != "a3" {
		t.Fatalf("turn2 (new) mismatch: %+v", turns[2])
	}
}

func TestLoadCodex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "019efd9f-2183-79b3-8c8d-7cef4444cc31"
	writeCodexLog(t, home, sid, []string{
		`{"type":"session_meta","payload":{"id":"` + sid + `","cwd":"/x"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"<permissions>"}]}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"build the app"}}`,
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"go build ./...\"}"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{}}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"working on it"}}`,
		`{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"Build passed."}}`,
	})

	turns, err := Load("Codex", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("want 1 turn, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "build the app" {
		t.Fatalf("prompt mismatch: %q", turns[0].Prompt)
	}
	// both the interim agent_message and the task_complete final are kept.
	if turns[0].Response != "working on it\n\nBuild passed." {
		t.Fatalf("response mismatch: %q", turns[0].Response)
	}
	if len(allSteps(turns[0])) != 1 || allSteps(turns[0])[0].Title != "exec" || allSteps(turns[0])[0].Detail != "go build ./..." {
		t.Fatalf("steps mismatch: %+v", allSteps(turns[0]))
	}
}

func TestLoadUnknownAgentOrMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if turns, err := Load("aider", "x", 10); err != nil || turns != nil {
		t.Fatalf("unknown agent should be (nil,nil), got %v %v", turns, err)
	}
	if turns, err := Load("claude", "does-not-exist", 10); err != nil || turns != nil {
		t.Fatalf("missing log should be (nil,nil), got %v %v", turns, err)
	}
	if turns, err := Load("claude", "", 10); err != nil || turns != nil {
		t.Fatalf("empty sessionID should be (nil,nil), got %v %v", turns, err)
	}
}

func TestLoadCodexResponseItemUserInput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "019efd9f-2183-79b3-8c8d-7cef4444cc32"
	writeCodexLog(t, home, sid, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions\\nrepo rules"}]}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>local</environment_context>"}]}}`,
		`{"type":"response_item","timestamp":"2026-09-27T10:00:00Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"build the app"}]}}`,
		`{"type":"event_msg","timestamp":"2026-09-27T10:00:00Z","payload":{"type":"user_message","message":"build the app"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"done"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"» gtmux·tick seq 1"}]}}`,
	})
	turns, err := Load("codex", sid, 10)
	if err != nil || len(turns) != 1 {
		t.Fatalf("turns=%+v err=%v; want one real user turn", turns, err)
	}
	if turns[0].Prompt != "build the app" || turns[0].Response != "done" {
		t.Errorf("turn=%+v; want the input and reply together", turns[0])
	}
}

func TestLoadCodexCurrentSanitizedFixture(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "fixture-session"
	b, err := os.ReadFile("testdata/codex-current.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".codex", "sessions", "2026", "09", "27")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-27T10-00-00-"+sid+".jsonl"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	turns, err := Load("codex", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "fix the parser" || turns[0].Response != "Checking the parser.\n\nThe parser is fixed." {
		t.Errorf("first turn = %+v", turns[0])
	}
	if turns[1].Prompt != "show me the tests" || turns[1].Response != "All tests pass." {
		t.Errorf("second turn = %+v", turns[1])
	}
	if got := len(allSteps(turns[0])); got != 1 {
		t.Errorf("first turn has %d tool steps, want 1", got)
	}
}

// TestLastMessageTime: the wall-clock of the LAST logged message (unix SECONDS),
// parsed from the log CONTENT — not the file mtime (which a resume bumps without
// adding messages).
func TestLastMessageTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaa1111-bbbb-cccc-dddd-eeeeeeeeeeee"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","timestamp":"2026-07-03T15:30:00.000Z","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","timestamp":"2026-07-03T15:33:53.950Z","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
	})
	// Bump the FILE mtime far past the last message — LastMessageTime must ignore it.
	log := filepath.Join(home, ".claude", "projects", "-some-project", sid+".jsonl")
	future := time.Unix(1800000000, 0)
	os.Chtimes(log, future, future)

	want, _ := time.Parse(time.RFC3339Nano, "2026-07-03T15:33:53.950Z")
	if got := LastMessageTime("claude", sid); got != want.Unix() {
		t.Errorf("LastMessageTime = %d, want the last message's timestamp %d (not the file mtime)", got, want.Unix())
	}
	if LastMessageTime("claude", "no-such-session") != 0 {
		t.Error("unknown session should return 0")
	}
}

// TestFirstMessageTime: the wall-clock of the FIRST logged message — how long the
// conversation has actually been running. It reads the log's HEAD, so it is independent of
// when any observer started watching: the HQ session-health sensor needs a serve restart NOT
// to reset a twelve-hour-old session's age to zero, which is exactly when a worn-out
// supervisor most needs to be told to rotate.
func TestFirstMessageTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "aaaa2222-bbbb-cccc-dddd-eeeeeeeeeeee"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","timestamp":"2026-07-03T09:00:00.000Z","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","timestamp":"2026-07-03T15:33:53.950Z","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
	})
	first, _ := time.Parse(time.RFC3339Nano, "2026-07-03T09:00:00.000Z")
	if got := FirstMessageTime("claude", sid); got != first.Unix() {
		t.Errorf("FirstMessageTime = %d, want the FIRST message's timestamp %d", got, first.Unix())
	}
	// The pair must disagree on a session with history — a FirstMessageTime that quietly
	// returned the tail would report every long session as brand new.
	if FirstMessageTime("claude", sid) == LastMessageTime("claude", sid) {
		t.Error("first and last must differ on a multi-message log")
	}
	if FirstMessageTime("claude", "no-such-session") != 0 {
		t.Error("unknown session should return 0")
	}
	if FirstMessageTime("claude", "") != 0 {
		t.Error("empty session should return 0")
	}
}

// LastMessageError: a session ENDED on an error iff its last real message is an
// isApiErrorMessage entry; mid-turn errors that recovered don't count.
func TestLastMessageError(t *testing.T) {
	errLine := `{"type":"assistant","isApiErrorMessage":true,"message":{"role":"assistant","content":"API Error: Unable to connect to API (UNKNOWN_CERTIFICATE_VERIFICATION_ERROR)"}}`
	userLine := `{"type":"user","message":{"role":"user","content":"go on"}}`
	okLine := `{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"All green."}]}}`

	cases := []struct {
		name    string
		lines   []string
		errored bool
		text    string
	}{
		{"ends on error", []string{userLine, errLine}, true,
			"Unable to connect to API (UNKNOWN_CERTIFICATE_VERIFICATION_ERROR)"},
		{"recovered mid-turn error", []string{errLine, okLine}, false, ""},
		{"normal end_turn", []string{userLine, okLine}, false, ""},
		{"error then meta-only tail (still errored)", []string{errLine,
			`{"type":"user","isMeta":true,"message":{"role":"user","content":"<command-name>/clear</command-name>"}}`}, true,
			"Unable to connect to API (UNKNOWN_CERTIFICATE_VERIFICATION_ERROR)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			writeClaudeLog(t, home, sid, c.lines)
			got, txt := LastMessageError("Claude Code", sid)
			if got != c.errored {
				t.Fatalf("errored = %v, want %v", got, c.errored)
			}
			if got && txt != c.text {
				t.Fatalf("text = %q, want %q", txt, c.text)
			}
		})
	}
}

func TestLastMessageErrorMissingLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, _ := LastMessageError("Claude Code", "no-such-session"); got {
		t.Fatal("missing log must not be errored")
	}
}

func TestErrorSummaryTruncates(t *testing.T) {
	long := "API Error: " + strings.Repeat("x", 200)
	got := errorSummary([]byte(`"` + long + `"`))
	if len([]rune(got)) > 90 || got[len(got)-3:] != "\xe2\x80\xa6" { // ends with …
		t.Fatalf("summary not truncated: %q (len=%d)", got, len(got))
	}
}

// opencode has no on-disk log — gtmux keeps its own transcript, written by
// AppendOpencode from the plugin-piped hook payload. This is the full round-trip:
// append user + assistant lines, then Load parses them into turns for the digest.
func TestLoadOpencodeRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "ses_3718401d3ffeLOahrVtyEuSOih"

	// no transcript yet → "no history", not an error.
	if turns, err := Load("opencode", sid, 10); err != nil || len(turns) != 0 {
		t.Fatalf("cold opencode load: turns=%v err=%v (want empty)", turns, err)
	}

	AppendOpencode(sid, "user", "fix the build")
	AppendOpencode(sid, "assistant", "Looking now")
	AppendOpencode(sid, "assistant", "Done — fixed it.")
	AppendOpencode(sid, "user", "now run tests")
	AppendOpencode(sid, "assistant", "All green.")
	// empties + unknown roles are dropped, never a stray turn.
	AppendOpencode(sid, "assistant", "   ")
	AppendOpencode(sid, "tool", "ignored role")
	AppendOpencode("", "user", "no session → dropped")

	turns, err := Load("opencode", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 turns, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "fix the build" || turns[0].Response != "Looking now\n\nDone — fixed it." {
		t.Fatalf("turn0 mismatch: %+v", turns[0])
	}
	if turns[1].Prompt != "now run tests" || turns[1].Response != "All green." {
		t.Fatalf("turn1 mismatch: %+v", turns[1])
	}
	// the digest's goal/last derive from these turns; the closing response is "last".
	if turns[1].Time == "" {
		t.Error("opencode turn should carry the append timestamp (LastMessageTime relies on it)")
	}
}

// The display name "opencode" (and the key) both resolve to the opencode parser.
func TestOpencodeNormalizeAgent(t *testing.T) {
	for _, name := range []string{"opencode", "opencode "} {
		if got := normalizeAgent(name); got != "opencode" {
			t.Errorf("normalizeAgent(%q)=%q, want opencode", name, got)
		}
	}
}

// A session that BEGAN with /clear must be recognisable as one. The chat view reads
// exactly one session, so a reset makes its history start over at zero — on 2026-08-09
// a 400-turn HQ shift was cleared and the phone showed three bubbles, which read as a
// bug until someone diffed the resume record's session id.
//
// The judgment is the FIRST non-meta user entry: the real log opens with the harness's
// meta caveat, then the slash invocation, and only then the conversation.
func TestSessionOriginDetectsAReset(t *testing.T) {
	caveat := `{"type":"user","isMeta":true,"timestamp":"2026-08-09T05:24:09.909Z","message":{"role":"user","content":"<local-command-caveat>Caveat: The messages below were generated by the user while running local commands.</local-command-caveat>"}}`
	for name, tc := range map[string]struct {
		lines    []string
		wantKind string
		wantAt   int64
	}{
		"cleared": {
			lines: []string{
				caveat,
				`{"type":"user","timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
				`{"type":"user","timestamp":"2026-08-09T05:24:27.463Z","message":{"role":"user","content":"» · gtmux·tick  seq 1-2"}}`,
			},
			wantKind: ResetClear,
			wantAt:   1786253049, // 2026-08-09T05:24:09Z = 13:24 local, the clear itself
		},
		"rotated with /new": {
			lines: []string{
				`{"type":"user","timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"<command-name>/new</command-name>"}}`,
			},
			wantKind: ResetNew,
			wantAt:   1786253049,
		},
		// An ordinary session opens with a real prompt — nothing was started over, so
		// there is nothing to announce.
		"ordinary session": {
			lines: []string{
				caveat,
				`{"type":"user","timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"ship it"}}`,
			},
		},
		// Another slash command is not a reset: /usage leaves the conversation intact,
		// so claiming one would explain a short history that has another cause.
		"opened with a different slash command": {
			lines: []string{
				`{"type":"user","timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"<command-name>/usage</command-name>"}}`,
			},
		},
		// A sidechain (sub-agent) entry is not the session's own opening.
		"sidechain first": {
			lines: []string{
				`{"type":"user","isSidechain":true,"timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
				`{"type":"user","timestamp":"2026-08-09T05:24:10.000Z","message":{"role":"user","content":"ship it"}}`,
			},
		},
		"empty log": {lines: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			writeClaudeLog(t, home, sid, tc.lines)
			kind, at := SessionOrigin("Claude Code", sid)
			if kind != tc.wantKind {
				t.Errorf("reset = %q; want %q", kind, tc.wantKind)
			}
			if at != tc.wantAt {
				t.Errorf("reset at = %d; want %d", at, tc.wantAt)
			}
		})
	}
}

// Only Claude Code's log records the invocation. For any other agent the honest answer
// is no claim at all — a hint the view then does not render, rather than a wrong one it
// does.
func TestSessionOriginMakesNoClaimForOtherAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	writeCodexLog(t, home, sid, []string{
		`{"type":"user","timestamp":"2026-08-09T05:24:09.506Z","message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
	})
	if kind, at := SessionOrigin("codex", sid); kind != "" || at != 0 {
		t.Errorf("SessionOrigin(codex) = (%q, %d); want no claim", kind, at)
	}
	if kind, _ := SessionOrigin("claude", "no-such-session"); kind != "" {
		t.Errorf("missing log claimed a reset: %q", kind)
	}
}

// A /compact writes a continuation message that LOOKS like a prompt: type "user", role
// "user", plain-string content beginning "This session is being continued…". Nobody typed
// it. Read as one, every surface shows the PREVIOUS session's summary as something the
// commander said — and a supervisor reading its own summary back as an instruction is a
// failure this fleet has already had (2026-08-03).
//
// The shape here is copied from two real logs on this machine: `isCompactSummary` is the
// only field that tells the entry apart.
func TestLoadClaudeSkipsTheCompactionSummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sid := "33333333-4444-5555-6666-777777777777"
	writeClaudeLog(t, home, sid, []string{
		`{"type":"user","message":{"role":"user","content":"real question one"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"an answer"}]}}`,
		`{"type":"user","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\nSummary:\n1. the commander asked for X"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"carrying on"}]}}`,
		`{"type":"user","message":{"role":"user","content":"real question two"}}`,
		`{"type":"assistant","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"second answer"}]}}`,
	})

	turns, err := Load("Claude Code", sid, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, tn := range turns {
		if strings.Contains(tn.Prompt, "being continued from a previous conversation") {
			t.Fatalf("the compaction summary was attributed to the user: %q", tn.Prompt)
		}
	}
	if len(turns) != 2 {
		t.Fatalf("want 2 real turns, got %d: %+v", len(turns), turns)
	}
	if turns[0].Prompt != "real question one" || turns[1].Prompt != "real question two" {
		t.Fatalf("real prompts lost or reordered: %+v", turns)
	}
	// Dropping the summary must not drop what came after it: the reply written while the
	// compacted turn was open belongs to the conversation.
	joined := turns[0].Response + turns[1].Response
	if !strings.Contains(joined, "carrying on") {
		t.Fatalf("the reply after the compaction was lost: %q", joined)
	}
}
