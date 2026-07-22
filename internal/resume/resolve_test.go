package resume

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTranscript plants a claude transcript for sid under a project dir, containing
// `cwd` — mirroring the real layout ~/.claude/projects/<encoded>/<sid>.jsonl.
func writeTranscript(t *testing.T, home, projectDir, sid, cwd string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// First line has NO cwd (the real store starts with a small header record), so the
	// scanner must look past it.
	body := `{"type":"last-prompt","sessionId":"` + sid + `"}` + "\n" +
		`{"type":"user","cwd":"` + cwd + `","sessionId":"` + sid + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The regression: the agent started in /a/b, then cd'd into /a/b/sub, so the record we
// captured points at the SUBdir — but the conversation is filed under /a/b, and only a
// resume from there finds it.
func TestResolveUsesTheDirTheConversationIsFiledUnder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const sid = "49ba091e-b995-40e2-9f39-27e80599039c"
	writeTranscript(t, home, "-Users-x-proj", sid, "/Users/x/proj")

	got, alive := Resolve(Record{Agent: "claude", SessionID: sid, Cwd: "/Users/x/proj/sub"})
	if !alive {
		t.Fatal("an existing conversation must be reported alive")
	}
	if got.Cwd != "/Users/x/proj" {
		t.Errorf("Cwd = %q; want the dir the transcript is filed under (/Users/x/proj), not the cd'd-into subdir", got.Cwd)
	}
}

// The second failure the user hit: the conversation is simply gone, so no directory
// would work — we must NOT emit a `--resume` that can only print an error.
func TestResolveReportsAMissingConversationDead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rec := Record{Agent: "claude", SessionID: "5facd6fa-4259-9449-b75e-738a6029dead", Cwd: "/Users/x/gone"}
	if _, alive := Resolve(rec); alive {
		t.Error("a conversation with no transcript on disk must be reported dead so restore skips it")
	}
}

// Agents whose store we don't inspect keep today's behavior exactly.
func TestResolveLeavesOtherAgentsAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	in := Record{Agent: "codex", SessionID: "abc", Cwd: "/Users/x/whatever"}
	got, alive := Resolve(in)
	if !alive || got != in {
		t.Errorf("Resolve(%+v) = %+v alive=%v; want it unchanged and alive", in, got, alive)
	}
}

// A transcript that exists but records no cwd: still alive, and we keep the caller's cwd
// rather than blanking it.
func TestResolveKeepsCallerCwdWhenTranscriptHasNone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const sid = "nocwd-0000-0000-0000-000000000000"
	dir := filepath.Join(home, ".claude", "projects", "-Users-x-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(`{"type":"last-prompt"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, alive := Resolve(Record{Agent: "claude", SessionID: sid, Cwd: "/Users/x/keep"})
	if !alive || got.Cwd != "/Users/x/keep" {
		t.Errorf("got %+v alive=%v; want the caller's cwd kept and alive", got, alive)
	}
}
