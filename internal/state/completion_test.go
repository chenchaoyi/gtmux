package state

import (
	"os"
	"testing"
	"time"
)

func TestCompletionRequiresProducerEvidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const pane = "%1"
	for _, contents := range []string{"", "inferred", `{"version":1,"at":1}`, `{"version":2,"agent":"claude","at":1}`} {
		if err := WriteMarker(FinishedPath(pane), contents); err != nil {
			t.Fatal(err)
		}
		if c, ok := ReadCompletion(pane); ok {
			t.Fatalf("display/invalid marker acquired authority: %+v", c)
		}
	}
	at := time.Now().Add(-time.Minute)
	if err := RecordCompletion(pane, "claude", "session", "turn", at); err != nil {
		t.Fatal(err)
	}
	// Repairing the idle-time cache must not re-date the lifecycle boundary.
	repaint := time.Now()
	if err := os.Chtimes(FinishedPath(pane), repaint, repaint); err != nil {
		t.Fatal(err)
	}
	if c, ok := ReadCompletion(pane); !ok || c.At != at.UnixNano() || c.SessionID != "session" || c.TurnID != "turn" {
		t.Fatalf("completion evidence changed with display mtime: %+v, %v", c, ok)
	}
}
