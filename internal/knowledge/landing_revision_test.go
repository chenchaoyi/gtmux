package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func pendingLandingFixture(t *testing.T) knowledgeOp {
	t.Helper()
	e := reviewFixture(t)
	if rc := CmdKnowledge([]string{"promote", e.ID, "--why", "synthetic carrier regression", "--for", AudienceHQ}); rc != 0 {
		t.Fatal("promote failed")
	}
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	return live[0]
}

// The JSON wire construction keeps this regression executable against the old
// implementation, which silently ignored the expected revision.
func TestLandingRejectsStaleReceiptAtCommit(t *testing.T) {
	e := pendingLandingFixture(t)
	b, err := json.Marshal(map[string]any{"op": knowledgeOpLand, "id": e.ID, "topic": e.Topic, "at": int64(12345), "ref": "synthetic:R0", "expected_revision": entryRevision(e)})
	if err != nil {
		t.Fatal(err)
	}
	var stale knowledgeOp
	if err := json.Unmarshal(b, &stale); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyReview(proposalFor(e), false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := commitKnowledgeOp(stale, "synthetic stale R0 receipt"); err == nil {
		t.Error("stale R0 receipt was accepted for R1")
	}
	after, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("stale receipt appended to ledger")
	}
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || !promotionPending(live[0]) {
		t.Fatalf("new revision was incorrectly settled: %+v", live)
	}
}

func landingOp(e knowledgeOp) knowledgeOp {
	return knowledgeOp{Op: knowledgeOpLand, ID: e.ID, Topic: e.Topic, At: time.Now().Unix(), Ref: "synthetic:receipt", ExpectedRevision: entryRevision(e)}
}

func TestLandingSerializesCarrierAndReceiptWithReview(t *testing.T) {
	e := pendingLandingFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	landed := make(chan error, 1)
	go func() {
		landed <- commitKnowledge(landingOp(e), nil, "barrier carry", func(current knowledgeOp) (string, error) {
			close(entered)
			<-release
			if entryRevision(current) != entryRevision(e) {
				return "", errors.New("published a different revision")
			}
			return carryIntoLocal(current)
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("carrier did not start")
	}
	started, reviewed := make(chan struct{}), make(chan error, 1)
	go func() { close(started); _, err := ApplyReview(proposalFor(e), false); reviewed <- err }()
	<-started
	select {
	case err := <-reviewed:
		t.Fatalf("review crossed the carrier/receipt boundary: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-landed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("carry deadlock")
	}
	select {
	case err := <-reviewed:
		if err == nil {
			t.Fatal("review accepted a stale pre-landing revision")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("review deadlock")
	}
	live, err := liveKnowledge()
	if err != nil || len(live) != 1 || live[0].LandedAt == 0 {
		t.Fatalf("landing did not commit: %v %v", live, err)
	}
	// Reviewing the now-landed revision must reopen it. A delayed R0 writer may
	// neither settle R1 nor overwrite the R1 carrier after its successful landing.
	if _, err := ApplyReview(proposalFor(live[0]), false); err != nil {
		t.Fatal(err)
	}
	live, err = liveKnowledge()
	if err != nil || !promotionPending(live[0]) {
		t.Fatalf("review did not reopen: %v", err)
	}
	if _, err := KnowledgeCarry(e.ID); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(LocalPath())
	if err != nil {
		t.Fatal(err)
	}
	published := false
	err = commitKnowledge(landingOp(e), nil, "delayed R0 carry", func(old knowledgeOp) (string, error) {
		published = true
		return carryIntoLocal(old)
	})
	if err == nil || published {
		t.Fatal("stale writer reached carrier publication")
	}
	after, err := os.ReadFile(LocalPath())
	if err != nil || !bytes.Equal(before, after) || !strings.Contains(string(after), live[0].Body) {
		t.Fatal("stale writer changed R1 carrier")
	}
}

func TestLandingCarrierFailureKeepsPendingAndRetryIsSafe(t *testing.T) {
	e := pendingLandingFixture(t)
	before, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("synthetic post-publication sync failure")
	err = commitKnowledge(landingOp(e), nil, "interrupted carry", func(current knowledgeOp) (string, error) {
		ref, err := carryIntoLocal(current)
		if err != nil {
			return ref, err
		}
		return ref, failure
	})
	var ce *CommitError
	if !errors.As(err, &ce) || ce.Committed || ce.Phase != "carrier" || !errors.Is(err, failure) {
		t.Fatalf("wrong recovery receipt: %v", err)
	}
	after, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed carrier committed a receipt")
	}
	live, err := liveKnowledge()
	if err != nil || !promotionPending(live[0]) {
		t.Fatal("failed carrier cleared pending")
	}
	if _, err := KnowledgeCarry(e.ID); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(LocalPath())
	if err != nil || strings.Count(string(content), localSection(e)) != 1 {
		t.Fatal("retry duplicated carrier")
	}
}

func TestLandingManualRevisionAndLegacyHistory(t *testing.T) {
	e := pendingLandingFixture(t)
	read, ok := KnowledgeEntry(e.ID)
	if !ok || read.Revision != entryRevision(e) {
		t.Fatal("detail omitted read revision")
	}
	if index := KnowledgeIndex(time.Now().Unix()); index.Entries[0].Revision != read.Revision {
		t.Fatal("index/detail revision mismatch")
	}
	before, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, rev := range []string{"", "stale"} {
		if err := KnowledgeLand(e.ID, "synthetic:manual", rev); err == nil {
			t.Fatalf("accepted revision %q", rev)
		}
	}
	after, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused manual receipt changed ledger")
	}
	if err := KnowledgeLand(e.ID, "synthetic:manual", read.Revision); err != nil {
		t.Fatal(err)
	}
	ops, err := readKnowledgeOps()
	if err != nil || ops[len(ops)-1].ExpectedRevision != read.Revision {
		t.Fatal("receipt lost its revision")
	}
	// Legacy receipts were unversioned; their historical meaning stays intact.
	legacy := foldKnowledge([]knowledgeOp{
		{Op: knowledgeOpAdd, ID: "legacy", Topic: "pitfalls", Title: "legacy"},
		{Op: knowledgeOpPromote, ID: "legacy", At: 10},
		{Op: knowledgeOpLand, ID: "legacy", At: 20, Ref: "old-runbook"},
	})
	if len(legacy) != 1 || legacy[0].LandedAt != 20 || legacy[0].LandedRef != "old-runbook" {
		t.Fatal("legacy history changed")
	}
}

func TestLandingCLIRequiresViewedRevision(t *testing.T) {
	e := pendingLandingFixture(t)
	if err := knowledgeLand([]string{e.ID, "--ref", "synthetic:manual"}); err == nil {
		t.Fatal("CLI accepted missing revision")
	}
	if err := knowledgeLand([]string{e.ID, "--ref", "synthetic:manual", "--revision", "old"}); err == nil {
		t.Fatal("CLI accepted stale revision")
	}
	if err := knowledgeLand([]string{e.ID, "--ref", "synthetic:manual", "--revision=" + entryRevision(e)}); err != nil {
		t.Fatal(err)
	}
	wire := goldenStdout(t, func() {
		if rc := CmdKnowledge([]string{"show", e.ID, "--json"}); rc != 0 {
			t.Error("CLI show failed")
		}
	})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(wire), &decoded); err != nil {
		t.Fatal(err)
	}
	current, _ := KnowledgeEntry(e.ID)
	if decoded["revision"] != current.Revision || decoded["body"] != e.Body || decoded["promotedAt"] != float64(e.PromotedAt) {
		t.Fatal("CLI read projection changed existing fields or lost revision")
	}
}
