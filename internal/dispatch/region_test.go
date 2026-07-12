package dispatch

import (
	"strings"
	"testing"
)

func TestSplitInputRegion_Box(t *testing.T) {
	cap := strings.Join([]string{
		"user: earlier message",
		"assistant: an earlier reply",
		"╭──────────────────────────────╮",
		"│ ❯ my draft text here          │",
		"╰──────────────────────────────╯",
	}, "\n")
	history, draft, structured := splitInputRegion(cap)
	if !structured {
		t.Fatalf("a box input must be structured")
	}
	if draft != "my draft text here" {
		t.Fatalf("draft = %q", draft)
	}
	if !strings.Contains(history, "earlier reply") || strings.Contains(history, "my draft text") {
		t.Fatalf("history leaked the draft or lost content: %q", history)
	}
}

func TestSplitInputRegion_EmptyDraftBox(t *testing.T) {
	cap := strings.Join([]string{
		"conversation line",
		"╭──────────────────────────────╮",
		"│ ❯                             │",
		"╰──────────────────────────────╯",
	}, "\n")
	history, draft, structured := splitInputRegion(cap)
	if !structured {
		t.Fatalf("a box input must be structured")
	}
	if draft != "" {
		t.Fatalf("empty draft box should yield empty draft, got %q", draft)
	}
	if !strings.Contains(history, "conversation line") {
		t.Fatalf("history missing: %q", history)
	}
}

func TestSplitInputRegion_NoBoxDegradesToPrompt(t *testing.T) {
	cap := "line one\nline two\n❯ typed text"
	history, draft, structured := splitInputRegion(cap)
	if !structured {
		t.Fatalf("a ❯ prompt must be structured")
	}
	if draft != "typed text" {
		t.Fatalf("no-box draft = %q", draft)
	}
	if !strings.Contains(history, "line two") {
		t.Fatalf("history = %q", history)
	}
}

func TestSplitInputRegion_PlainShellIsUnstructured(t *testing.T) {
	// A bare shell prompt has no locatable input region → structured=false, so the
	// deliver guard won't wipe the pasted text as a "fragment".
	cap := "user@host project % echo hello world"
	_, draft, structured := splitInputRegion(cap)
	if structured {
		t.Fatalf("a plain shell prompt should be unstructured")
	}
	if draft != "" {
		t.Fatalf("unstructured draft should be empty, got %q", draft)
	}
}

func TestIsBoxBorder(t *testing.T) {
	for _, ok := range []string{"╭──────╮", "╰──────╯", "──────────", "├────┤"} {
		if !isBoxBorder(ok) {
			t.Fatalf("should be a border: %q", ok)
		}
	}
	for _, no := range []string{"│ ❯ hello │", "just text", "│", "─"} {
		if isBoxBorder(no) {
			t.Fatalf("should NOT be a border: %q", no)
		}
	}
}
