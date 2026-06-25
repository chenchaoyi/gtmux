package resume

import (
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	key := "my-sess:1.2" // a tmux locator with a colon — must survive as a filename
	want := Record{Agent: "claude", SessionID: "abc-123", Cwd: "/tmp/proj", UpdatedAt: 42}
	if err := Save(key, want); err != nil {
		t.Fatal(err)
	}
	got, ok := Load(key)
	if !ok || got != want {
		t.Fatalf("Load = %+v, %v; want %+v", got, ok, want)
	}
	Remove(key)
	if _, ok := Load(key); ok {
		t.Error("record should be gone after Remove")
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, ok := Load("nope:0.0"); ok {
		t.Error("missing key should not load")
	}
}

func TestKeyWithSlashIsSafe(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// A session named with a slash must not escape the resume dir.
	key := "feat/x:1.1"
	if err := Save(key, Record{Agent: "codex", SessionID: "z"}); err != nil {
		t.Fatalf("save with slash key: %v", err)
	}
	if _, ok := Load(key); !ok {
		t.Error("slash-bearing key should round-trip")
	}
}

func TestCommandPerAgent(t *testing.T) {
	cases := []struct {
		agent, id, want string
	}{
		{"claude", "s1", "claude --resume 's1'"},
		{"codex", "s2", "codex resume 's2'"},
		{"cursor", "s3", "cursor-agent --resume 's3'"},
		{"gemini", "s4", "gemini --resume 's4'"},
		{"kiro", "s5", "kiro-cli chat --resume-id 's5'"},
		{"opencode", "s6", "opencode --session 's6'"},
		{"grok", "s7", "grok -r 's7'"},
	}
	for _, c := range cases {
		got, ok := Command(Record{Agent: c.agent, SessionID: c.id})
		if !ok || got != c.want {
			t.Errorf("Command(%s) = %q,%v; want %q", c.agent, got, ok, c.want)
		}
	}
}

func TestCommandCwdGuard(t *testing.T) {
	got, ok := Command(Record{Agent: "claude", SessionID: "id", Cwd: "/a/b c"})
	if !ok {
		t.Fatal("want resumable")
	}
	if !strings.HasPrefix(got, "{ cd -- '/a/b c' 2>/dev/null || [ ! -d '/a/b c' ]; } && ") {
		t.Errorf("missing cwd guard: %q", got)
	}
	if !strings.HasSuffix(got, "claude --resume 'id'") {
		t.Errorf("missing resume tail: %q", got)
	}
}

func TestCommandNotResumable(t *testing.T) {
	if _, ok := Command(Record{Agent: "claude", SessionID: ""}); ok {
		t.Error("no session id → not resumable")
	}
	if _, ok := Command(Record{Agent: "unknown-agent", SessionID: "x"}); ok {
		t.Error("unknown agent → not resumable")
	}
	if Resumable("unknown-agent") {
		t.Error("Resumable(unknown) should be false")
	}
}

func TestCommandQuoteEscape(t *testing.T) {
	// A session id with a single quote must be escaped, not break the command.
	got, _ := Command(Record{Agent: "claude", SessionID: "a'b"})
	if !strings.Contains(got, `'a'\''b'`) {
		t.Errorf("single quote not escaped: %q", got)
	}
}
