package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestWorkspaceHoldsBeforeResolvingOrTypingWhenWakeOwnsInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	defer release()
	// No real HQ lookup or tmux call is necessary when the input channel is busy.
	got := workspaceDeliver(hqworkspace.Conversation{}, "saved user instruction")
	if !got.SafeHold || got.State != "hq-input-locked" {
		t.Fatal("competing wake input was not safely held", got)
	}
}

func TestWorkspaceRequestIsOneStrictBoundedJSONAndPreservesMarkdown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	body := "**user**\n`$(echo should-not-run)`"
	input := `{"op":"send","id":"request-id","body":"**user**\n` + "`$(echo should-not-run)`" + `"}`
	a, err := readWorkspaceAction(strings.NewReader(input))
	if err != nil || a.Body != body {
		t.Fatal(a, err)
	}
	for _, invalid := range []string{input + input, `{"op":"send","boddy":"bad"}`, strings.Repeat(" ", 65537)} {
		if _, err := readWorkspaceAction(strings.NewReader(invalid)); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	s := hqworkspace.New(t.TempDir())
	if _, err := applyWorkspaceAction(s, a, 100); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Read()
	if d.Messages[a.ID].Body != body || d.Messages[a.ID].Delivery != "pending" {
		t.Fatal("read/mutation unexpectedly drove HQ")
	}
	if _, err := applyWorkspaceAction(s, workspaceAction{Op: "report", ID: "report-id", Kind: "progress", Body: "fake HQ"}, 101); err == nil {
		t.Fatal("non HQ published report")
	}
}
func TestWorkspaceReplyRequiresExactSavedInputNotTitleOrQuotedMarker(t *testing.T) {
	m := hqworkspace.Message{ID: "request-id", Body: "check it"}
	turns := []transcript.Turn{{Prompt: "[gtmux workspace message request-id; matter foo (renamed)]\ncheck it", Response: "real"}}
	if r := workspaceReply(m, turns); r == nil || r.Response != "real" {
		t.Fatal("explicit witness not matched")
	}
	bad := []transcript.Turn{{Prompt: "other", Response: turns[0].Prompt}, {Prompt: "[gtmux workspace message request-id; wrong]\nchanged body"}}
	if r := workspaceReply(m, bad); r != nil {
		t.Fatal("quoted marker or changed prompt matched")
	}
	d := hqworkspace.Empty()
	d.Messages[m.ID] = m
	if got := workspaceUntrackedTurns(append(turns, bad...), d); len(got) != len(bad) {
		t.Fatal("ordinary prose hidden", got)
	}
}
func TestWorkspaceOnlyFoldsConfirmedWakePayload(t *testing.T) {
	line := "» gtmux·tick seq 1-2 · #a3f1c2"
	turns := []transcript.Turn{{Prompt: line}, {Prompt: "I saw " + line}, {Prompt: line + "\nuser instruction"}}
	records := []events.Record{{Event: events.AuditEventWakeDelivered, Summary: line}}
	got := workspaceClassifyTurns(turns, records)
	if !got[0].Signal || got[1].Signal || got[2].Signal {
		t.Fatal("signal classifier hid user intent", got)
	}
	if workspaceClassifyTurns(turns, nil)[0].Signal {
		t.Fatal("unconfirmed signal folded")
	}
}
func TestSpawnMatterValidationBeforeAnyLaunch(t *testing.T) {
	s := hqworkspace.New(t.TempDir())
	if err := validateSpawnMatter(s, "absent"); err == nil {
		t.Fatal("unknown goal accepted")
	}
	_, _ = s.Create("matter-a", "Existing", 100)
	if err := validateSpawnMatter(s, "matter-a"); err != nil {
		t.Fatal(err)
	}
	_ = s.Update("matter-a", "", "closed", "", 0, 101)
	if err := validateSpawnMatter(s, "matter-a"); err == nil {
		t.Fatal("closed goal accepted")
	}
}

func TestWorkspaceSourceBudgetWarnsInsteadOfTruncatingAReply(t *testing.T) {
	out := workspaceDetailReport{Turns: []workspaceTurn{{Turn: transcript.Turn{Prompt: "huge", Response: strings.Repeat("a", 600<<10)}}, {Turn: transcript.Turn{Prompt: "small", Response: "kept"}}}, Warnings: []string{}}
	boundWorkspaceTurns(&out)
	if len(out.Turns) != 1 || out.Turns[0].Response != "kept" || len(out.Warnings) == 0 {
		t.Fatal("oversized source was silently shortened", out)
	}
}

func TestStructuredProgressDoesNotHideLaterPublicCommentary(t *testing.T) {
	report := hqworkspace.Message{Speaker: "hq", ReplyTo: "request-a", Body: "Checking now"}
	source := transcript.Turn{Response: "Checking now\n\nFinished", Segments: []transcript.Segment{{Text: "Checking now"}, {Text: "Finished"}}}
	remaining := workspaceDistinctReply(&source, "request-a", map[string]hqworkspace.Message{"report-a": report})
	if remaining == nil || remaining.Response != "Finished" || len(remaining.Segments) != 1 {
		t.Fatal("later public reply suppressed", remaining)
	}
	if source.Segments[0].Text != "Checking now" {
		t.Fatal("source mutated")
	}
}

func TestWorkspaceDispatchCannotAdoptLaterOccupantOfItsOldPane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := hqworkspace.New(t.TempDir())
	_, _ = s.Create("matter-a", "Task", 100)
	c := workspaceCandidate{Conversation: hqworkspace.Conversation{Agent: "codex", HomeID: "home", SessionID: "later-session", Name: "same title", Pane: "%10", Loc: "same:0.0"}}
	_ = resume.Save(c.Loc, resume.Record{Agent: "codex", HomeID: "home", SessionID: c.SessionID, UpdatedAt: 200})
	task := dispatch.Task{ID: "task-one", MatterID: "matter-a", MatterSessionID: "original-session", MatterHomeID: "home", Pane: "%10", Agent: "codex", Delivered: true, CreatedAt: 100}
	if linked, err := linkWorkspaceTask(s, task, []workspaceCandidate{c}, 201); linked || err != nil {
		t.Fatal("new occupant adopted", linked, err)
	}
	task.MatterSessionID = c.SessionID
	if linked, err := linkWorkspaceTask(s, task, []workspaceCandidate{c}, 201); !linked || err != nil {
		t.Fatal("exact saved source not linked", linked, err)
	}
	d, _ := s.Read()
	if d.Matters["matter-a"].Participants[0].Conversation.SessionID != c.SessionID {
		t.Fatal("wrong source")
	}
}

func TestWorkspaceDeliveryDoesNotBlockFeedTickOrStartConcurrentWorkers(t *testing.T) {
	var runner workspaceRunner
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	runner.start(func() { close(started); <-release; close(done) })
	<-started
	runner.start(func() { t.Error("two background senders started") })
	close(release)
	<-done
}
