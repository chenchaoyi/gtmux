package dispatch

import (
	"os"
	"strings"
	"testing"
)

func TestPayloadHash_StableAndDistinct(t *testing.T) {
	a := PayloadHash("%1", "hello")
	if a != PayloadHash("%1", "hello") {
		t.Fatalf("hash must be stable")
	}
	if a == PayloadHash("%2", "hello") {
		t.Fatalf("different pane must differ")
	}
	if a == PayloadHash("%1", "hellox") {
		t.Fatalf("different text must differ")
	}
}

func TestIsDuplicate_Window(t *testing.T) {
	pane, text := "%1", "run the build"
	h := PayloadHash(pane, text)
	recent := func(string) (string, int64) { return h, 100 }

	if !isDuplicate(recent, pane, text, 130, 60) {
		t.Fatalf("identical payload within window should be a duplicate")
	}
	if isDuplicate(recent, pane, text, 200, 60) {
		t.Fatalf("after the window lapses it is not a duplicate")
	}
	if isDuplicate(recent, pane, "different", 130, 60) {
		t.Fatalf("a different payload is never a duplicate")
	}
	if isDuplicate(recent, pane, text, 130, 0) {
		t.Fatalf("window<=0 disables the interlock")
	}
}

func TestRecentSend_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if h, ts := RecentSend("%9"); h != "" || ts != 0 {
		t.Fatalf("empty store should return zero, got %q %d", h, ts)
	}
	RecordSend("%9", "deadbeef", 12345)
	if h, ts := RecentSend("%9"); h != "deadbeef" || ts != 12345 {
		t.Fatalf("round-trip failed: %q %d", h, ts)
	}
}

func TestSanitizePane(t *testing.T) {
	if got := sanitizePane("%12"); got != "%12" {
		t.Fatalf("pane id mangled: %q", got)
	}
	// A traversal attempt is reduced to a contained basename (no separators).
	got := sanitizePane("../etc/passwd")
	if got != "passwd" || strings.ContainsRune(got, os.PathSeparator) {
		t.Fatalf("path traversal not contained: %q", got)
	}
}
