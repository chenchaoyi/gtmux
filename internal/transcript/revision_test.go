package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A rewrite that keeps the size, and an atomic replace, are changes: the parse cache used
// to serve the old turns for both, and LogRevision (the HTTP ETag) kept its value, since
// both judged a log by its size alone (%12, 2026-10-06). Growing and shrinking still work.
func TestTheCacheAndTheRevisionSeeEveryKindOfChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "audit-session"
	path := filepath.Join(dir, sid+".jsonl")
	line := func(prompt string) []byte {
		return []byte(`{"type":"user","message":{"role":"user","content":"` + prompt + `"}}` + "\n")
	}
	prompt := func() string {
		turns, err := Load("claude", sid, 10)
		if err != nil || len(turns) == 0 {
			t.Fatalf("Load = %v, %v", turns, err)
		}
		return turns[len(turns)-1].Prompt
	}
	base := time.Now().Add(-time.Hour).Truncate(time.Second)

	if err := os.WriteFile(path, line("old-value"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(path, base, base)
	if got := prompt(); got != "old-value" {
		t.Fatalf("first read = %q", got)
	}
	rev := LogRevision("claude", sid)

	// Rewritten in place, the same size, a second later.
	if err := os.WriteFile(path, line("new-value"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(path, base.Add(time.Second), base.Add(time.Second))
	if got := prompt(); got != "new-value" {
		t.Errorf("after a same-size rewrite: %q, want new-value", got)
	}
	if r := LogRevision("claude", sid); r == rev {
		t.Error("the revision did not move for a same-size rewrite")
	} else {
		rev = r
	}

	// Replaced atomically by a file of the same size AND the same mtime: only its
	// identity differs.
	tmp := filepath.Join(dir, "tmp.jsonl.part")
	if err := os.WriteFile(tmp, line("rep-value"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(tmp, base.Add(time.Second), base.Add(time.Second))
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	if got := prompt(); got != "rep-value" {
		t.Errorf("after an atomic replace: %q, want rep-value", got)
	}
	if r := LogRevision("claude", sid); r == rev {
		t.Error("the revision did not move for an atomic replace")
	}

	// The controls: an append extends, a shorter rewrite starts over.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.Write(line("appended"))
	_ = f.Close()
	if got := prompt(); got != "appended" {
		t.Errorf("after an append: %q", got)
	}
	if err := os.WriteFile(path, line("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := prompt(); got != "s" {
		t.Errorf("after a shorter rewrite: %q", got)
	}
}
