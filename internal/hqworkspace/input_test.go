package hqworkspace

import "testing"

func TestHumanQuestionRemainsSeparateFromReportsAndGoal(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Create("goal-id", "Synthetic goal", 100)
	q := UserInput{ID: "question-id", Kind: "decision", Prompt: "Keep the sample?", Why: "Two valid options remain.", Impact: "Keeping uses 1 MiB; removal needs a separate action.", Choices: []string{"Keep", "Remove"}}
	if e := s.RequestInput("goal-id", q, 0, 101); e != nil {
		t.Fatal(e)
	}
	if e := s.RequestInput("goal-id", q, 0, 102); e != nil {
		t.Fatal("idempotency", e)
	}
	_, e := s.Publish("progress-id", "goal-id", "", "progress", "Long technical report does not replace the question.", "", 103)
	if e != nil {
		t.Fatal(e)
	}
	d, _ := s.Read()
	o := BuildOverview(d, 104)
	if o.NeedsInput[0].Preview.Body != q.Prompt {
		t.Fatal(o)
	}
	if _, e = s.SubmitReply("answer-id", "goal-id", "Keep", q.ID, 105); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SubmitReply("answer-id", "goal-id", "Keep", q.ID, 106); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SubmitReply("other-answer", "goal-id", "Remove", q.ID, 107); e == nil {
		t.Fatal("double answer accepted")
	}
	d, _ = s.Read()
	if d.Messages["answer-id"].Delivery != "pending" || d.Matters["goal-id"].State != "needs-input" || d.Matters["goal-id"].UserInput.AnswerID != "answer-id" {
		t.Fatal("saved answer misrepresented as executed")
	}
	if e = s.ClearInput("goal-id", "wrong-id", "clear-id", "Verified synthetic receipt", 0, 108); e == nil {
		t.Fatal("stale question cleared")
	}
	if e = s.ClearInput("goal-id", q.ID, "clear-id", "Verified synthetic receipt", d.Matters["goal-id"].Revision, 108); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	if d.Matters["goal-id"].State != "open" || d.Matters["goal-id"].UserInput != nil || d.Messages[q.ID].UserInput == nil || len(d.Messages) != 4 {
		t.Fatal("goal or audit lost")
	}
}
func TestHumanRequestBoundsAndStaleReply(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Create("goal-id", "Goal", 100)
	if e := s.RequestInput("goal-id", UserInput{ID: "request-id", Kind: "decision", Prompt: "Empty reason"}, 0, 101); e == nil {
		t.Fatal("unclear question accepted")
	}
	if _, e := s.SubmitReply("answer-id", "goal-id", "yes", "old-id", 102); e == nil {
		t.Fatal("unrelated answer linked")
	}
	d, _ := s.Read()
	if len(d.Messages) != 0 || d.Matters["goal-id"].State != "open" {
		t.Fatal("failed action mutated records")
	}
}

func TestOverviewSeparatesHumanQuestionsFromHQClarificationAndSavedReplies(t *testing.T) {
	d := Empty()
	d.Matters["legacy"] = Matter{ID: "legacy", State: "needs-input"}
	d.Matters["answered"] = Matter{ID: "answered", State: "needs-input", UserInput: &UserInput{ID: "question-answered", Prompt: "Answered?", AnswerID: "answer-id"}}
	d.Matters["current"] = Matter{ID: "current", State: "needs-input", UserInput: &UserInput{ID: "question-current", Prompt: "Choose one?"}}
	out := BuildOverview(d, 100)
	if len(out.HumanInput) != 1 || out.HumanInput[0].MatterID != "current" || len(out.AwaitingHQ) != 2 {
		t.Fatal(out)
	}
	if len(out.NeedsInput) != 3 {
		t.Fatal("older client projection lost")
	}
}

func TestCancellingUnsentAnswerRestoresTheQuestionWithoutDeletingHistory(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Create("goal-id", "Goal", 100)
	q := UserInput{ID: "question-id", Kind: "manual", Prompt: "Open the sample?", Why: "A local operator is required.", Impact: "Only the sample is opened."}
	if e := s.RequestInput("goal-id", q, 0, 101); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SubmitReply("answer-id", "goal-id", "Later", q.ID, 102); e != nil {
		t.Fatal(e)
	}
	if e := s.Cancel("answer-id", 103); e != nil {
		t.Fatal(e)
	}
	d, _ := s.Read()
	if d.Matters["goal-id"].UserInput.AnswerID != "" || d.Messages["answer-id"].Delivery != "cancelled" {
		t.Fatal("cancelled reply still hides the question")
	}
}

func TestMergingMattersPreservesPendingQuestionAndCancelledReply(t *testing.T) {
	s := New(t.TempDir())
	_, _ = s.Create("source-id", "Source", 100)
	_, _ = s.Create("target-id", "Target", 100)
	q := UserInput{ID: "question-id", Kind: "decision", Prompt: "Keep?", Why: "A preference is needed.", Impact: "Only the synthetic sample."}
	if e := s.RequestInput("source-id", q, 0, 101); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SubmitReply("answer-id", "source-id", "Keep", q.ID, 102); e != nil {
		t.Fatal(e)
	}
	if e := s.Merge("source-id", "target-id", 0, 103); e != nil {
		t.Fatal(e)
	}
	if e := s.Cancel("answer-id", 104); e != nil {
		t.Fatal(e)
	}
	d, _ := s.Read()
	m := d.Matters["target-id"]
	if m.UserInput == nil || m.UserInput.ID != q.ID || m.UserInput.AnswerID != "" || m.State != "needs-input" {
		t.Fatal("request lost during merge", m)
	}
}
