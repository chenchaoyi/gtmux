package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCHH005_StatCachesAndO1AgySessionTitle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexDir := filepath.Join(home, ".codex")
	t.Setenv("CODEX_HOME", codexDir)
	agyDir := filepath.Join(home, ".gemini", "antigravity-cli")
	t.Setenv("AGY_HOME", agyDir)

	// 1) claudeLogPath + LastMessageError cache hit and invalidation on file growth.
	projDir := filepath.Join(home, ".claude", "projects", "proj-a")
	if err := os.MkdirAll(projDir, 0o700); err != nil {
		t.Fatal(err)
	}
	claudeFile := filepath.Join(projDir, "claude-sess-1.jsonl")
	errLine := `{"type":"assistant","isApiErrorMessage":true,"message":{"role":"assistant","content":"API Error: 529 overloaded"}}` + "\n"
	if err := os.WriteFile(claudeFile, []byte(errLine), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := claudeLogPath("claude-sess-1"); got != claudeFile {
		t.Fatalf("claudeLogPath = %q, want %q", got, claudeFile)
	}
	if isErr, msg := LastMessageError("claude", "claude-sess-1"); !isErr || msg == "" {
		t.Fatalf("expected initial LastMessageError=true, got (%v, %q)", isErr, msg)
	}

	// Append recovery assistant turn -> size changes -> LastMessageError cache invalidates.
	f, err := os.OpenFile(claudeFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"assistant","message":{"role":"assistant","content":"Recovered cleanly"}}` + "\n")
	_ = f.Close()
	if isErr, msg := LastMessageError("claude", "claude-sess-1"); isErr {
		t.Fatalf("stale LastMessageError cache after file growth: (%v, %q)", isErr, msg)
	}

	// 2) SessionTitles("codex") stat cache hit + invalidation on append.
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(codexDir, "session_index.jsonl")
	if err := os.WriteFile(idxPath, []byte(`{"id":"cx-1","thread_name":"First Title"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := SessionTitles("codex")["cx-1"]; got != "First Title" {
		t.Fatalf("SessionTitles initial = %q, want First Title", got)
	}
	idxF, err := os.OpenFile(idxPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = idxF.WriteString(`{"id":"cx-1","thread_name":"Renamed Title"}` + "\n")
	_ = idxF.Close()
	if got := SessionTitles("codex")["cx-1"]; got != "Renamed Title" {
		t.Fatalf("SessionTitles after append = %q, want Renamed Title", got)
	}

	// 3) AgySessionTitle reads only the single target .pbtxt file in O(1).
	annDir := filepath.Join(agyDir, "annotations")
	if err := os.MkdirAll(annDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(annDir, "target-conv.pbtxt"), []byte(`title:"Target Agy Session"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := AgySessionTitle("target-conv"); got != "Target Agy Session" {
		t.Fatalf("AgySessionTitle = %q, want Target Agy Session", got)
	}
}
