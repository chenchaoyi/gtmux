package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCodexBindingReadsOnlySubmittedUserMessages(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	wire := "Repair it\n\n" + SessionBindingPrefix + strings.Repeat("a", 32)
	line := func(kind, role, text string) string {
		b, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": kind, "role": role, "content": []any{map[string]string{"type": "input_text", "text": text}}}})
		return string(b)
	}
	legacy, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "user_message", "message": wire}})
	writeCodexLog(t, home, "binding-reader", []string{
		"malformed JSON", line("message", "assistant", wire), line("function_call_output", "user", wire),
		line("message", "user", wire), string(legacy), line("message", "user", wire+"\nextra"),
	})
	if got := CodexBindingPrompts("binding-reader"); !reflect.DeepEqual(got, []string{wire, wire}) {
		t.Fatalf("wrong submitted witnesses: %+v", got)
	}
	if clean, ok := CleanUserPrompt(wire); !ok || clean != "Repair it" {
		t.Fatalf("metadata leaked into parsed Chat: %q/%v", clean, ok)
	}
	malformed := "User text\n" + SessionBindingPrefix + "not-a-token"
	if clean, _ := CleanUserPrompt(malformed); clean != malformed {
		t.Fatal("ordinary user text was removed", clean)
	}
}

func TestCodexBindingReadsBoundedRolloutTail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	writeCodexLog(t, home, "binding-tail", nil)
	path := codexLogPath("binding-tail")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	// The first witness falls outside the bound and must not be used. The
	// final witness follows a partial line at the seek boundary and survives.
	old := "Old\n" + SessionBindingPrefix + strings.Repeat("a", 32)
	wire := "New\n" + SessionBindingPrefix + strings.Repeat("b", 32)
	write := func(prompt string) {
		b, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "user_message", "message": prompt}})
		f.Write(append(b, '\n'))
	}
	write(old)
	block := strings.Repeat("x", 4095) + "\n"
	for i := 0; i < 2100; i++ {
		if _, err := f.WriteString(block); err != nil {
			t.Fatal(err)
		}
	}
	write(wire)
	f.Close()
	if got := CodexBindingPrompts("binding-tail"); !reflect.DeepEqual(got, []string{wire}) {
		t.Fatalf("bounded witness scan = %+v", got)
	}
	// A continuation with another session's metadata never lends its payload.
	other := filepath.Join(filepath.Dir(path), "rollout-2026-10-03T00-00-00-binding-tail_other.jsonl")
	if err := os.WriteFile(other, []byte(`{"type":"session_meta","payload":{"id":"other"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := CodexBindingPrompts("binding-tail"); !reflect.DeepEqual(got, []string{wire}) {
		t.Fatal("foreign continuation affected binding", got)
	}
}
