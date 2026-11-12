package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func dreamFixture(t *testing.T, n int, calls int) DreamRun {
	t.Helper()
	asHQ(t)
	for i := 0; i < n; i++ {
		e := knowledgeOp{Op: knowledgeOpAdd, ID: fmt.Sprintf("pitfalls/review-%d", i), Topic: "pitfalls", Title: "Verify the current input", Body: "Keep the original evidence and check current applicability.", Hits: 4}
		if err := commitKnowledgeOp(e, "synthetic dream fixture"); err != nil {
			t.Fatal(err)
		}
	}
	r, err := StartDream(DreamStart{Selection: DreamRequest{Mode: "pilot", Limit: n}, MaxCalls: calls, MaxSeconds: 100})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func dreamChoice(model string) DreamChoice {
	return DreamChoice{Agent: "codex", Model: model, Reason: "HQ selected this model for evidence review", TimeoutSeconds: 10}
}

func keepDream(call DreamCall) DreamReply {
	d := DreamDecision{ID: call.Item.ID, BaseRevision: call.Item.Revision, Disposition: "keep", Note: "The supplied evidence supports retaining the bounded rule."}
	b, _ := json.Marshal(d)
	return DreamReply{Text: string(b), UsageKnown: true, InputTokens: 100, GenerationTokens: 50, UsageSource: "synthetic:terminal_usage"}
}

func TestDreamResumeRetainsCompletedItemsAndModelHistory(t *testing.T) {
	r := dreamFixture(t, 2, 10)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	calls := []string{}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("missing simulated controller interruption")
			}
		}()
		_, _ = ResumeDream(context.Background(), r.ID, dreamChoice("first-model"), func(_ context.Context, call DreamCall) (DreamReply, error) {
			calls = append(calls, call.Item.ID)
			if len(calls) == 2 {
				panic("controller disappeared after persisted start")
			}
			return keepDream(call), nil
		})
	}()
	status, err := DreamStatus(r.ID)
	if err != nil || status.Active == nil || len(status.Decisions) != 1 {
		t.Fatalf("lost progress: %+v %v", status, err)
	}
	resumed := 0
	runner := func(_ context.Context, call DreamCall) (DreamReply, error) {
		resumed++
		if call.Item.ID == calls[0] {
			t.Error("completed item replayed")
		}
		return keepDream(call), nil
	}
	status, err = ResumeDream(context.Background(), r.ID, dreamChoice("second-model"), runner)
	if err == nil || resumed != 0 || status.UnknownUsage != 1 {
		t.Fatalf("ambiguous request silently retried: %+v %v", status, err)
	}
	status, err = ResumeDream(context.Background(), r.ID, dreamChoice("second-model"), runner)
	if err != nil || resumed != 1 || status.State != "review_ready" || status.Calls != 3 || status.InputTokens != 200 {
		t.Fatalf("resume: %+v %v", status, err)
	}
	history, err := DreamHistory(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	models := []string{}
	for _, e := range history {
		if e.Call != nil {
			models = append(models, e.Call.Choice.Model)
		}
	}
	if strings.Join(models, ",") != "first-model,first-model,second-model" {
		t.Fatal(models)
	}
	if _, err = ResumeDream(context.Background(), r.ID, dreamChoice("unused-model"), runner); err != nil || resumed != 1 {
		t.Fatal("finished run called model again", err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("dream wrote KB or hits")
	}
	if st, err := os.Stat(dreamJournalPath()); err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("private archive mode", st, err)
	}
}

func TestDreamConcurrentResumeAndSourceChanges(t *testing.T) {
	r := dreamFixture(t, 1, 5)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, c DreamCall) (DreamReply, error) {
			close(entered)
			<-release
			return keepDream(c), nil
		})
		done <- err
	}()
	<-entered
	if _, err := ResumeDream(context.Background(), r.ID, dreamChoice("other"), func(context.Context, DreamCall) (DreamReply, error) {
		t.Error("concurrent model call")
		return DreamReply{}, nil
	}); err == nil {
		t.Error("second runner accepted")
	}
	live, _ := liveKnowledge()
	p := proposalFor(live[0])
	if _, err := ApplyReview(p, false); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("stale result accepted")
	}
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("after-edit"), func(context.Context, DreamCall) (DreamReply, error) {
		t.Error("changed evidence sent to model")
		return DreamReply{}, nil
	})
	if err != nil || status.State != "review_ready" || len(status.Decisions) != 1 || status.Decisions[0].Disposition != "defer" {
		t.Fatal(status, err)
	}
}

func TestDreamBudgetsPrivacyAndCorruptionStopBeforeCalls(t *testing.T) {
	r := dreamFixture(t, 2, 1)
	count := 0
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, c DreamCall) (DreamReply, error) { count++; return keepDream(c), nil })
	if err == nil || count != 1 || status.State != "paused" || len(status.Pending) != 1 {
		t.Fatal(status, err)
	}
	if _, err := ResumeDream(context.Background(), r.ID, dreamChoice("new-model"), func(context.Context, DreamCall) (DreamReply, error) { count++; return DreamReply{}, nil }); err == nil || count != 1 {
		t.Fatal("model switch reset call limit")
	}
	original, _ := os.ReadFile(dreamJournalPath())
	if err := os.WriteFile(dreamJournalPath(), original[:len(original)-1], 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DreamStatus(r.ID); err == nil {
		t.Fatal("truncated journal accepted")
	}
	if _, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(context.Context, DreamCall) (DreamReply, error) {
		t.Error("called on corrupt history")
		return DreamReply{}, nil
	}); err == nil {
		t.Fatal("corrupt journal resumed")
	}
	if err := os.WriteFile(dreamJournalPath(), original, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDreamPrivateReclassificationAndInvalidResults(t *testing.T) {
	r := dreamFixture(t, 1, 5)
	if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpSensitive, ID: r.Plan.Items[0].ID, Sensitive: true}, "synthetic privacy change"); err != nil {
		t.Fatal(err)
	}
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(context.Context, DreamCall) (DreamReply, error) {
		t.Error("private input sent")
		return DreamReply{}, nil
	})
	if err != nil || status.Calls != 0 || status.Decisions[0].Disposition != "defer" {
		t.Fatal(status, err)
	}
	item := r.Plan.Items[0]
	for _, text := range []string{`{"id":"wrong","base_revision":"wrong","disposition":"keep","note":"ok"}`, `{} {}`} {
		if _, err := parseDreamDecision(text, item, nil); err == nil {
			t.Fatal("invalid identity/JSON accepted")
		}
	}
}

func TestDreamModelProposalsRemainInertAndStatusIsReadOnly(t *testing.T) {
	r := dreamFixture(t, 1, 5)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	status, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, c DreamCall) (DreamReply, error) {
		d := DreamDecision{ID: c.Item.ID, BaseRevision: c.Item.Revision, Disposition: "revise", Note: "Clarify the trigger.", Change: &ReviewChange{ID: c.Item.ID, BaseRevision: c.Item.Revision, Operation: "revise", Title: "Verify input before reuse", Body: "Verify current applicability and retain the original evidence.", Why: "Clarify the trigger", Evidence: []string{"kb:" + c.Item.ID + "@" + c.Item.Revision}}}
		b, _ := json.Marshal(d)
		return DreamReply{Text: string(b)}, nil
	})
	if err != nil || status.UnknownUsage != 1 {
		t.Fatal(status, err)
	}
	p, err := DreamProposal(r.ID)
	if err != nil || len(p.Changes) != 1 {
		t.Fatal(p, err)
	}
	if result, err := ApplyReview(p, true); err != nil || result.Committed {
		t.Fatal(result, err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("automatic proposal applied itself")
	}
	journal, _ := os.ReadFile(dreamJournalPath())
	if _, err := DreamRuns(); err != nil {
		t.Fatal(err)
	}
	if _, err := DreamHistory(r.ID); err != nil {
		t.Fatal(err)
	}
	final, _ := os.ReadFile(dreamJournalPath())
	if !bytes.Equal(journal, final) {
		t.Fatal("status/history changed archive")
	}
}

func TestDreamSharedLifetimeBudgetAndUnresolvedRun(t *testing.T) {
	t.Run("lifetime", func(t *testing.T) {
		r := dreamFixture(t, 2, 10)
		count := 0
		_, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, c DreamCall) (DreamReply, error) {
			count++
			reply := keepDream(c)
			reply.InputTokens = 500000
			return reply, nil
		})
		if err == nil || count != 1 {
			t.Fatal("reported limit did not stop next call", err)
		}
		// Age this synthetic history across a week boundary. All hashes are
		// recomputed deliberately; this is not a production recovery operation.
		events, err := readDreamEvents()
		if err != nil {
			t.Fatal(err)
		}
		var data []byte
		previous := ""
		for _, e := range events {
			e.At -= 8 * 24 * 3600
			e.Previous = previous
			e.Hash = dreamEventHash(e)
			previous = e.Hash
			b, _ := json.Marshal(e)
			data = append(data, append(b, '\n')...)
		}
		if err := os.WriteFile(dreamJournalPath(), data, 0600); err != nil {
			t.Fatal(err)
		}
		next, err := StartDream(DreamStart{Selection: DreamRequest{Mode: "pilot", IDs: []string{r.Plan.Items[1].ID}}, MaxCalls: 10, MaxSeconds: 100})
		if err != nil {
			t.Fatal(err)
		}
		_, err = ResumeDream(context.Background(), next.ID, dreamChoice("other"), func(context.Context, DreamCall) (DreamReply, error) { count++; return DreamReply{}, nil })
		if err == nil || count != 1 {
			t.Fatal("new packet/week reset pilot allowance", err)
		}
	})
	t.Run("unresolved", func(t *testing.T) {
		r := dreamFixture(t, 2, 10)
		func() {
			defer func() { _ = recover() }()
			_, _ = ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(context.Context, DreamCall) (DreamReply, error) { panic("controller loss") })
		}()
		next, err := StartDream(DreamStart{Selection: DreamRequest{Mode: "pilot", IDs: []string{r.Plan.Items[1].ID}}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = ResumeDream(context.Background(), next.ID, dreamChoice("other"), func(context.Context, DreamCall) (DreamReply, error) {
			t.Error("launched with an unresolved run")
			return DreamReply{}, nil
		})
		if err == nil {
			t.Fatal("unresolved attempt bypassed")
		}
		if _, err := DeferDream(r.ID, r.Plan.Items[0].ID, "pending inspection"); err == nil {
			t.Fatal("defer erased unresolved attempt")
		}
	})
}

func TestDreamExplicitDeferRetainsFailedAttempts(t *testing.T) {
	r := dreamFixture(t, 2, 10)
	count := 0
	for i := 0; i < 4; i++ {
		_, err := ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(context.Context, DreamCall) (DreamReply, error) {
			count++
			return DreamReply{}, fmt.Errorf("synthetic failure")
		})
		if err == nil {
			t.Fatal("failed attempt not paused")
		}
	}
	if count != 3 {
		t.Fatal("retry bound", count)
	}
	if _, err := DeferDream(r.ID, r.Plan.Items[0].ID, ""); err == nil {
		t.Fatal("unexplained defer")
	}
	s, err := DeferDream(r.ID, r.Plan.Items[0].ID, "HQ will investigate missing evidence separately")
	if err != nil || s.Calls != 3 || s.UnknownUsage != 3 || len(s.Pending) != 1 {
		t.Fatal(s, err)
	}
	s, err = ResumeDream(context.Background(), r.ID, dreamChoice("chosen"), func(_ context.Context, c DreamCall) (DreamReply, error) { return keepDream(c), nil })
	if err != nil || s.State != "review_ready" || s.Calls != 4 || s.Decisions[0].Disposition != "defer" {
		t.Fatal(s, err)
	}
	if _, err := DeferDream(r.ID, r.Plan.Items[0].ID, "again"); err == nil {
		t.Fatal("completed item deferred again")
	}
}

func TestDreamRejectsInventedEvidenceAndJournalIdentity(t *testing.T) {
	r := dreamFixture(t, 1, 10)
	item := r.Plan.Items[0]
	ops, _ := readKnowledgeOps()
	d := DreamDecision{ID: item.ID, BaseRevision: item.Revision, Disposition: "revise", Note: "Review proposal", Change: &ReviewChange{ID: item.ID, BaseRevision: item.Revision, Operation: "revise", Title: "Check current input", Body: item.Body, Why: "Clarify", Evidence: []string{"invented-source"}}}
	b, _ := json.Marshal(d)
	if _, err := parseDreamDecision(string(b), item, ops); err == nil {
		t.Fatal("invented evidence accepted")
	}
	d.Change.Evidence = []string{"kb:" + item.ID + "@" + item.Revision}
	d.Change.Memory = &MemoryRule{Confidence: "supported", VerifiedAt: 1}
	b, _ = json.Marshal(d)
	if _, err := parseDreamDecision(string(b), item, ops); err == nil {
		t.Fatal("model invented verification")
	}
	call := DreamCall{RunID: r.ID, Attempt: 1, Item: item, Choice: dreamChoice("chosen")}
	events, _ := readDreamEvents()
	events = append(events, DreamEvent{RunID: r.ID, Kind: "started", Call: &call}, DreamEvent{RunID: r.ID, Kind: "finished", Attempt: 1, Decision: &DreamDecision{ID: "wrong", BaseRevision: item.Revision}})
	if _, err := foldDream(events, r.ID); err == nil {
		t.Fatal("mismatched completion accepted")
	}
}
