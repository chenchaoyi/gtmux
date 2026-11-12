package knowledge

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func reviewFixture(t *testing.T) knowledgeOp {
	t.Helper()
	asHQ(t)
	op := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/write-check", Topic: "pitfalls", Title: "check written bytes", Body: "Compare source and target after an authorized replacement.", Hits: 4, Status: StatusHypothesis, Sources: []Candidate{{ID: "evidence-1", Lesson: "source survives"}}}
	if err := commitKnowledgeOp(op, "fixture"); err != nil {
		t.Fatal(err)
	}
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	return live[0]
}

func proposalFor(e knowledgeOp) ReviewProposal {
	p := ReviewProposal{Version: 1, Origin: "review", Changes: []ReviewChange{{ID: e.ID, BaseRevision: entryRevision(e), Operation: "revise", Title: "verify written content", Body: "Compare content before depending on the new file; command success is insufficient.", Why: "clarify the trigger and verification", Evidence: []string{"synthetic:test"}}}}
	p.ID = proposalDigest(p)
	return p
}

func TestReviewAtomicConflictAndIdempotentRetry(t *testing.T) {
	e := reviewFixture(t)
	p := proposalFor(e)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	bad := p
	bad.Changes = append([]ReviewChange{}, p.Changes...)
	bad.Changes[0].BaseRevision = "stale"
	bad.ID = proposalDigest(bad)
	if _, err := ApplyReview(bad, false); err == nil {
		t.Fatal("accepted stale revision")
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("conflict changed ledger")
	}
	r, err := ApplyReview(p, false)
	if err != nil || !r.Committed {
		t.Fatalf("apply: %+v %v", r, err)
	}
	first, _ := os.ReadFile(knowledgeLedgerPath())
	r, err = ApplyReview(p, false)
	if err != nil || !r.AlreadyApplied {
		t.Fatalf("retry: %+v %v", r, err)
	}
	second, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(first, second) {
		t.Fatal("retry appended again")
	}
	live, _ := liveKnowledge()
	if len(live) != 1 || live[0].ID != e.ID || live[0].Hits != 4 || live[0].Status != StatusHypothesis || len(live[0].Sources) != 1 {
		t.Fatalf("lost identity/observations/hypothesis/evidence: %+v", live)
	}
}

func TestReviewRejectsWholeBatchBeforeAnyAppend(t *testing.T) {
	e := reviewFixture(t)
	p := proposalFor(e)
	p.Changes = append(p.Changes, ReviewChange{ID: "missing", Operation: "retire", BaseRevision: "missing", Why: "obsolete", Evidence: []string{"test"}})
	p.ID = proposalDigest(p)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	if _, err := ApplyReview(p, false); err == nil {
		t.Fatal("accepted partially valid batch")
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("partial batch committed")
	}
}

func TestReviewConcurrentRevisionHasOneWinner(t *testing.T) {
	e := reviewFixture(t)
	a, b := proposalFor(e), proposalFor(e)
	b.Changes[0].Body = "Another concurrently reviewed correction."
	b.ID = proposalDigest(b)
	results := make(chan error, 2)
	for _, p := range []ReviewProposal{a, b} {
		go func(p ReviewProposal) { _, err := ApplyReview(p, false); results <- err }(p)
	}
	wins := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent writers produced %d winners", wins)
	}
	live, _ := liveKnowledge()
	if len(live) != 1 || live[0].Hits != e.Hits {
		t.Fatal("lost entry or observations")
	}
}

func TestReviewDryRunWritesNothingAndProtectedRetireRefused(t *testing.T) {
	e := reviewFixture(t)
	p := proposalFor(e)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	if r, err := ApplyReview(p, true); err != nil || r.Committed {
		t.Fatalf("dry run: %+v %v", r, err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(before, after) {
		t.Fatal("dry run wrote ledger")
	}
	e.Provenance = ProvCorrection
	p.Changes[0].Operation = "retire"
	p.Changes[0].BaseRevision = entryRevision(e)
	p.ID = proposalDigest(p)
	_, _, err := prepareReview(p, []knowledgeOp{e}, 1)
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("retired correction: %v", err)
	}
	e.Memory = nil
	e.Audience = AudienceHQ
	p = proposalFor(e)
	p.Changes[0].Memory = &MemoryRule{Actions: []string{"review"}, Summary: "check", Scope: "global", Confidence: "supported", Evidence: []string{"test"}}
	p.ID = proposalDigest(p)
	if _, _, err := prepareReview(p, []knowledgeOp{e}, 10); err == nil {
		t.Fatal("ordinary review widened the declared audience")
	}
}

func TestReviewedRevisionReopensCarrierAndDoesNotRefreshVerification(t *testing.T) {
	e := reviewFixture(t)
	e.Memory = &MemoryRule{Actions: []string{"copy"}, Summary: "check bytes", Scope: "local", Confidence: "supported", Evidence: []string{"test"}, VerifiedAt: 1, ProtectedReason: "avoid silent replacement failure"}
	e.PromotedAt, e.LandedAt, e.Audience, e.LandedRef = 2, 3, AudienceHQ, "old carrier"
	p := proposalFor(e)
	prepared, _, err := prepareReview(p, []knowledgeOp{e}, 10)
	if err != nil {
		t.Fatal(err)
	}
	live := foldKnowledge(append([]knowledgeOp{e}, prepared...))
	if !promotionPending(live[0]) || live[0].Audience != AudienceHQ || live[0].LandedAt != 0 {
		t.Fatalf("new revision claims old landing: %+v", live[0])
	}
	if live[0].Memory.Confidence != "tentative" || live[0].Memory.VerifiedAt != 0 || e.Memory.VerifiedAt != 1 {
		t.Fatal("unreviewed card became verified or mutated source")
	}
}
