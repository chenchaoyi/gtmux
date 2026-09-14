package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

// On 2026-09-02 the supervisor's pane sat in copy-mode for 1h40m with 22 wakes queued
// correctly behind the guard. Three surfaces noticed and none named the cause: "hooks
// installed but not firing — restart the agent", "check the input box for a stuck
// draft", "not landing, reconcile by pull". Every word true; none of it the one keypress
// that fixed it. These tests are about whether the reader learns what to DO.
func TestReachSentenceNamesTheCauseAndTheFix(t *testing.T) {
	copyMode := reachSentence("%6", dispatch.ReachCopyMode, false)
	if !strings.Contains(copyMode, "%6") {
		t.Errorf("copy-mode = %q — it must name WHICH pane", copyMode)
	}
	if !strings.Contains(copyMode, "copy-mode") || !strings.Contains(copyMode, "q") {
		t.Errorf("copy-mode = %q — it must name the mode and the key that leaves it", copyMode)
	}
	if strings.Contains(copyMode, "draft") {
		t.Errorf("copy-mode = %q — still pointing at a draft that is not there", copyMode)
	}

	draft := reachSentence("%6", dispatch.ReachDraft, false)
	if !strings.Contains(draft, "unsent") || strings.Contains(draft, "copy-mode") {
		t.Errorf("draft = %q", draft)
	}

	unreadable := reachSentence("%6", dispatch.ReachUnreadable, false)
	if !strings.Contains(unreadable, "%6") || strings.Contains(unreadable, "copy-mode") {
		t.Errorf("unreadable = %q", unreadable)
	}

	// Reachable is silence: a row that always says something says nothing when it counts.
	if s := reachSentence("%6", "", true); s != "" {
		t.Errorf("a reachable pane produced %q", s)
	}
}

func TestHookSilenceNoteKeepsTheRightAdviceForEachPane(t *testing.T) {
	// A pane in copy-mode is silent for a reason hooks have nothing to do with, and
	// "restart the agent" spends a live session on a wrong diagnosis.
	scrolled := hookSilenceNote([]string{"%6", "%9"})
	if !strings.Contains(scrolled, "%6") || !strings.Contains(scrolled, "%9") {
		t.Errorf("note = %q — it must name which panes are scrolled", scrolled)
	}
	if !strings.Contains(scrolled, "copy-mode") {
		t.Errorf("note = %q — the cause is missing", scrolled)
	}
	// …and the hook advice must survive for the panes it still applies to.
	if !strings.Contains(scrolled, "restart") {
		t.Errorf("note = %q — the others are still a hook problem", scrolled)
	}

	plain := hookSilenceNote(nil)
	if strings.Contains(plain, "copy-mode") || !strings.Contains(plain, "restart") {
		t.Errorf("with nothing scrolled, the note = %q — it must be the unchanged hook advice", plain)
	}
}

// The hook-silence judgment, pinned on the 2026-09-14 reading: fourteen sessions restored
// after a reboot, each with one SessionStart 8.6 hours old and nothing since, must not be
// named — they are idle, not mute — while a pane whose last word opened a turn and then
// went quiet past the grace must be.
func TestHookSilenceJudgesByTheHooksLastWordNotByPainting(t *testing.T) {
	now := int64(1_800_000_000)
	h := int64(3600)
	last := map[string]events.Record{}
	restored := []string{"%1", "%3", "%4", "%7", "%9", "%11", "%13", "%15", "%16", "%17", "%18", "%19", "%22", "%27"}
	for _, id := range restored {
		last[id] = events.Record{Pane: id, Ts: now - int64(8.6*float64(h)), Event: "SessionStart", State: "idle"}
	}
	// The three that were genuinely talking: each ended its last turn, or is mid-turn
	// within the grace.
	last["%20"] = events.Record{Pane: "%20", Ts: now - 5*60, Event: "Stop", State: "idle"}
	last["%6"] = events.Record{Pane: "%6", Ts: now - 20*60, Event: "UserPromptSubmit", State: "working"}
	last["%29"] = events.Record{Pane: "%29", Ts: now - 3*h, Event: "Stop", State: "idle"}
	// The incident shape: a turn opened, then nothing for hours.
	last["%40"] = events.Record{Pane: "%40", Ts: now - 6*h, Event: "UserPromptSubmit", State: "working"}
	// Waiting on a person is not silence.
	last["%41"] = events.Record{Pane: "%41", Ts: now - 6*h, Event: "Waiting", State: "waiting"}

	panes := append(append([]string{}, restored...), "%20", "%6", "%29", "%40", "%41", "%99")
	got := hookSilentPanes(last, panes, now)
	if len(got) != 1 || got[0] != "%40 (6h)" {
		t.Fatalf("silent = %v, want only %%40 (6h): restored-and-idle panes, finished turns, a turn inside the grace, a wait, and a pane with no event are all not silence", got)
	}
}
