package hq

import (
	"os"
	"testing"
)

// A capture parses "<lesson> @<topic>", writes one well-formed spool line with a dedup
// key, and auto-collects the pane context.
func TestCaptureWritesSpoolLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "%42")
	t.Setenv("GTMUX_TASK_ID", "t-xyz")

	if rc := CmdCapture([]string{"wrangler TLS-resets from the office; retry @pitfalls"}); rc != 0 {
		t.Fatalf("capture rc = %d, want 0", rc)
	}

	cands, err := readCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(cands))
	}
	c := cands[0]
	if c.Topic != "pitfalls" {
		t.Errorf("topic = %q, want pitfalls", c.Topic)
	}
	if c.Lesson != "wrangler TLS-resets from the office; retry" {
		t.Errorf("lesson = %q (the @topic must be stripped)", c.Lesson)
	}
	if c.Key == "" || c.Key[:9] != "pitfalls/" {
		t.Errorf("key = %q, want a pitfalls/<slug> dedup key", c.Key)
	}
	if c.Pane != "%42" || c.Task != "t-xyz" {
		t.Errorf("context not captured: pane=%q task=%q", c.Pane, c.Task)
	}
}

// An unknown or missing @topic errors and writes nothing.
func TestCaptureRejectsBadTopic(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if rc := CmdCapture([]string{"a lesson with no topic"}); rc == 0 {
		t.Error("capture with no @topic should be non-zero")
	}
	if rc := CmdCapture([]string{"a lesson @nonsense"}); rc != 2 {
		t.Errorf("capture with an unknown topic rc = %d, want 2", rc)
	}
	if _, err := os.Stat(pendingDistillPath()); !os.IsNotExist(err) {
		t.Error("a rejected capture must not create the spool")
	}
}

// Two phrasings of the same fact collide on the dedup key so distill can merge them.
func TestCaptureDedupKeyCollides(t *testing.T) {
	if got := slug("wrangler TLS resets from the office"); got != slug("wrangler TLS resets from the office!!!") {
		t.Errorf("trailing punctuation must not change the slug: %q", got)
	}
	// The key is capped to the first few words so an incidental trailing clause doesn't split it.
	a := slug("dispatch fast ops separately from slow ones always")
	b := slug("dispatch fast ops separately from slow ones because they hide")
	if a != b {
		t.Errorf("first-6-words cap should collide these: %q vs %q", a, b)
	}
}

// --list renders the pending queue and is empty on a fresh home.
func TestCaptureList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if rc := CmdCapture([]string{"--list"}); rc != 0 {
		t.Errorf("empty --list rc = %d, want 0", rc)
	}
	if rc := CmdCapture([]string{"release flow: tag then wait for CI @workflows"}); rc != 0 {
		t.Fatal("capture failed")
	}
	cands, _ := readCandidates()
	if len(cands) != 1 || cands[0].Topic != "workflows" {
		t.Fatalf("queue = %+v", cands)
	}
}
