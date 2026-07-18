package dispatch

import (
	"strings"
	"testing"
)

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
	if !ContainsHead(screen, text) {
		t.Fatalf("re-wrapped head should still match")
	}
}

func TestContainsHead_HaystackNotTruncated(t *testing.T) {
	// The fingerprint falls past the haystack's own first 40 runes — it must still
	// match because the haystack is not truncated (regression guard).
	text := "state machine with layered checks and evidence"
	screen := "me: implement the verified dispatch " + text
	if !ContainsHead(screen, text) {
		t.Fatalf("head past the 40-rune mark should still match")
	}
}

func TestContainsHead_EmptyNeverMatches(t *testing.T) {
	if ContainsHead("anything at all", "   ") {
		t.Fatalf("a blank delivery must match nothing")
	}
}

func TestNormalizeTail_IsTheTrailingFingerprint(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOP" // > headRunes
	tail := NormalizeTail(long)
	if r := []rune(tail); len(r) != headRunes {
		t.Fatalf("want %d runes, got %d", headRunes, len(r))
	}
	if !strings.HasSuffix(long, tail) {
		t.Fatalf("tail must be the END of the payload; got %q", tail)
	}
	if NormalizeTail(long) == NormalizeHead(long) {
		t.Fatalf("for a long payload head and tail must differ")
	}
}

func TestNormalizeTail_ShortPayloadEqualsHead(t *testing.T) {
	// A payload no longer than headRunes has head == tail (the whole thing), so a
	// head match already implies the tail — the tail check is a no-op, not a regression.
	short := "run make check"
	if NormalizeTail(short) != NormalizeHead(short) {
		t.Fatalf("short payload: head and tail must be identical")
	}
}

func TestContainsTail_MatchesFullDraftNotHeadOnly(t *testing.T) {
	text := "implement the verified dispatch state machine with layered checks and evidence"
	headOnly := "me: " + NormalizeHead(text) // the first frame: only the head rendered
	full := "me: " + text
	if ContainsTail(headOnly, text) {
		t.Fatalf("a head-only draft must NOT satisfy the tail check (would submit truncated)")
	}
	if !ContainsTail(full, text) {
		t.Fatalf("the full draft must satisfy the tail check")
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
