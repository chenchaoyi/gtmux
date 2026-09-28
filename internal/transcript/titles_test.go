package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionTitles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "session_index.jsonl")
	if got := SessionTitles("codex"); len(got) != 0 {
		t.Fatalf("missing index: %v", got)
	}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"id":"one","thread_name":"Original"}
malformed
{"id":"two","thread_name":"Other conversation"}
{"id":"one","thread_name":"  Renamed\n session  "}
{"id":"clear","thread_name":"Old name"}
{"id":"clear","thread_name":""}
{"id":"one","thread_name":`)
	got := SessionTitles("Codex")
	if got["one"] != "Renamed session" || got["two"] != "Other conversation" || got["clear"] != "" || got["missing"] != "" {
		t.Fatalf("titles: %v", got)
	}
	if got := SessionTitles("claude"); len(got) != 0 {
		t.Fatalf("must not read another agent's index: %v", got)
	}
	write(`{"id":"one","thread_name":"Changed on next poll"}`)
	if got := SessionTitles("codex")["one"]; got != "Changed on next poll" {
		t.Fatalf("stale title: %q", got)
	}
}

func TestSessionTitlesBoundedTail(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	data := `{"id":"old","thread_name":"Outside the tail"}` + "\n" +
		strings.Repeat("x", titleIndexLimit) + "\n" +
		`{"id":"recent","thread_name":"Recent title"}` + "\n"
	if err := os.WriteFile(filepath.Join(codexHome(), "session_index.jsonl"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got := SessionTitles("codex")
	if len(got) != 1 || got["recent"] != "Recent title" {
		t.Fatalf("bounded tail: %v", got)
	}
}
