package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexClientUsesSessionOriginator(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "09", "29")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ id, originator, source, want string }{
		{"desktop", "codex_work_desktop", "vscode", "chatgpt_desktop"},
		{"terminal", "codex-tui", "vscode", "terminal"},
		{"unrecognized", "future-client", "cli", ""},
	} {
		path := filepath.Join(dir, "rollout-2026-09-29T00-00-00-"+tc.id+".jsonl")
		data := `{"type":"session_meta","payload":{"id":"` + tc.id + `","originator":"` + tc.originator + `","source":"` + tc.source + `"}}` + "\n"
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := CodexClient(tc.id); got != tc.want {
			t.Errorf("CodexClient(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
	if got := CodexClient("missing"); got != "" {
		t.Errorf("missing client = %q", got)
	}
}

func TestCodexClientRejectsMismatchedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "archived_sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-09-29T00-00-00-target.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"other","originator":"codex_work_desktop"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := CodexClient("target"); got != "" {
		t.Errorf("mismatched client = %q", got)
	}
}
