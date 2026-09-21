package hq

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// The supervisor's pull view hides gtmux's audit trail, and the audit trail is the only
// thing that says who wrote a prompt. Those two facts together are issue #1156: the
// distinction existed but was invisible without `--all`.
//
// They have to COMPOSE, not trade off. The view still hides the evidence — nothing about
// what HQ is shown or owes may change — and the author is worked out from the delta
// BEFORE the hiding, so the answer survives the view that withholds its proof.
func TestTheAuthorSurvivesTheViewThatHidesTheEvidence(t *testing.T) {
	// RESOLVED: on macOS t.TempDir() hands back /var/folders/… while os.Getwd() reports
	// /private/var/folders/…, and fromHQHome compares the two for equality.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	hq := state.HQHome()
	if mkErr := os.MkdirAll(hq, 0o755); mkErr != nil {
		t.Fatal(mkErr)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if cdErr := os.Chdir(hq); cdErr != nil {
		t.Fatal(cdErr)
	}
	t.Setenv("TMUX_PANE", "%20") // HQ's own pane, whose echo the view also drops

	line := "动手吧。这条是我替司令答的，他预先授权了「需要就修」。"
	delta := []events.Record{
		{Seq: 10, Event: "UserPromptSubmit", Pane: "%18", Origin: events.OriginInstruction, Summary: line},
		{Seq: 11, Event: events.AuditEventSend, Pane: "%18", Actor: "hq", Summary: "landed: " + line},
		{Seq: 12, Event: "UserPromptSubmit", Pane: "%18", Origin: events.OriginInstruction, Summary: "这句是司令自己敲的"},
	}

	// Worked out from the whole delta, exactly as the command does before it hides.
	author := events.AuthorOf(delta)
	if author[10] != "hq" {
		t.Fatalf("the relayed prompt was not attributed: %q", author[10])
	}
	if who, ok := author[12]; ok {
		t.Errorf("the commander's own line was credited to %q", who)
	}

	shown, hidden := pullView(delta, true)
	if hidden == 0 {
		t.Fatal("the view hid nothing — this test is not exercising it")
	}
	for _, r := range shown {
		if events.IsAudit(r) {
			t.Error("the audit trail is being SHOWN; the pull view's contract changed")
		}
	}
	// And the answer is still there for the records that remain.
	for _, r := range shown {
		if r.Seq == 10 && author[r.Seq] != "hq" {
			t.Error("the author was lost along with the evidence")
		}
	}
}

// Filtered and --all reads are untouched: the view only applies to the unfiltered pull,
// which is the one that counts as consumption.
func TestTheViewStillOnlyAppliesToTheUnfilteredPull(t *testing.T) {
	delta := []events.Record{{Seq: 1, Event: events.AuditEventSend, Pane: "%18", Actor: "hq", Summary: "landed: x"}}
	shown, hidden := pullView(delta, false)
	if hidden != 0 || len(shown) != 1 {
		t.Errorf("a non-applying view changed the read: shown=%d hidden=%d", len(shown), hidden)
	}
}
