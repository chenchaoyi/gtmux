package relay

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestOfflineRequestSurvivesAndRetryDoesNotDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := Request{ID: "req-42", Kind: "ask", Pane: "%7", Session: "work", TaskID: "t1", Body: "Need a decision", For: "user", Blocking: true}
	first, fresh, err := Put(in)
	if err != nil || !fresh {
		t.Fatalf("first put: %v %v", fresh, err)
	}
	second, fresh, err := Put(in)
	if err != nil || fresh || second.CreatedAt != first.CreatedAt {
		t.Fatalf("retry: %#v %v %v", second, fresh, err)
	}
	if len(List()) != 1 {
		t.Fatal("retry made another request")
	}
	in.Body = "different"
	if _, _, err = Put(in); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed payload: %v", err)
	}
}

func TestClaimRotationAndReplyReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := Request{ID: "req-43", Kind: "ask", Pane: "%7", Session: "work", Body: "Question", For: "hq", Blocking: true}
	if _, _, err := Put(in); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(in.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(in.ID); !errors.Is(err, ErrState) {
		t.Fatalf("duplicate claim: %v", err)
	}
	err := transaction(func() error {
		r, _ := read(in.ID)
		r.ClaimedAt = time.Now().Add(-6 * time.Minute).Unix()
		return write(r)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(in.ID); err != nil {
		t.Fatalf("new HQ could not reclaim: %v", err)
	}
	r, err := Resolve(in.ID, "Ask the user to approve this in their own pane.")
	if err != nil || r.Delivered {
		t.Fatalf("reply persisted before delivery: %#v %v", r, err)
	}
	if _, err := Resolve(in.ID, "different"); !errors.Is(err, ErrState) {
		t.Fatalf("changed reply: %v", err)
	}
	if err := MarkDelivered(in.ID); err != nil {
		t.Fatal(err)
	}
	r, err = Get(in.ID)
	if err != nil || !r.Delivered || r.Pane != "%7" || r.Session != "work" {
		t.Fatalf("receipt/routing: %#v %v", r, err)
	}
}

func TestReportCanCloseWithoutReply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := Request{ID: "req-44", Kind: "report", Pane: "%9", Session: "work", Body: "Tests passing", For: "hq"}
	if _, _, err := Put(in); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(in.ID); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(in.ID, "")
	if err != nil || r.State != "closed" {
		t.Fatalf("close: %#v %v", r, err)
	}
	if err := MarkDelivered(in.ID); !errors.Is(err, ErrState) {
		t.Fatalf("closed report delivered: %v", err)
	}
}

func TestUserDirectedQuestionCannotBeAnsweredByHQ(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := Request{ID: "req-user", Kind: "ask", Pane: "%9", Session: "work", Body: "Approve install?", For: "user", Blocking: true}
	if _, _, err := Put(in); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(in.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(in.ID, "Approved"); err == nil {
		t.Fatal("HQ approved a user decision")
	}
	r, _ := Get(in.ID)
	if r.State != "claimed" || r.Reply != "" {
		t.Fatalf("permission boundary changed request: %#v", r)
	}
}

func TestCorruptRequestIsNeverOverwrittenOnRetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := Request{ID: "req-corrupt", Kind: "report", Pane: "%9", Session: "work", Body: "Update", For: "hq"}
	if _, _, err := Put(in); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path(in.ID), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Put(in); err == nil {
		t.Fatal("corrupt record was overwritten")
	}
	b, _ := os.ReadFile(path(in.ID))
	if string(b) != "{broken" {
		t.Fatalf("record changed: %q", b)
	}
}
