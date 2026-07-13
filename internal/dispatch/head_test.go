package dispatch

import "testing"

func TestNormalizeHead_CollapsesAndTruncates(t *testing.T) {
	got := NormalizeHead("  hello\n  world\t\tfoo  ")
	if got != "hello world foo" {
		t.Fatalf("collapse failed: %q", got)
	}
	long := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"
	if r := []rune(NormalizeHead(long)); len(r) != headRunes {
		t.Fatalf("want %d runes, got %d", headRunes, len(r))
	}
}

func TestContainsHead_SurvivesReWrap(t *testing.T) {
	text := "please refactor the delivery verifier to prefer hook evidence"
	// The TUI re-wrapped the line and padded it; the head must still match.
	screen := "user asked:\n  please refactor the delivery\n  verifier to prefer hook evidence\n"
	if !containsHead(screen, text) {
		t.Fatalf("re-wrapped head should still match")
	}
}

func TestContainsHead_HaystackNotTruncated(t *testing.T) {
	// The fingerprint falls past the haystack's own first 40 runes — it must still
	// match because the haystack is not truncated (regression guard).
	text := "state machine with layered checks and evidence"
	screen := "me: implement the verified dispatch " + text
	if !containsHead(screen, text) {
		t.Fatalf("head past the 40-rune mark should still match")
	}
}

func TestContainsHead_EmptyNeverMatches(t *testing.T) {
	if containsHead("anything at all", "   ") {
		t.Fatalf("a blank delivery must match nothing")
	}
}

func TestLooksQueued(t *testing.T) {
	if !looksQueued("...\nPress up to edit queued messages\n❯") {
		t.Fatalf("should detect the queued indicator")
	}
	if looksQueued("just a normal screen") {
		t.Fatalf("false positive on a normal screen")
	}
}
