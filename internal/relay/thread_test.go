package relay

import (
	"errors"
	"testing"
)

func TestThreadOnlySourceAndMutationBoundary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := Request{ID: "thread-request", Kind: "ask", Agent: "codex", HomeID: "synthetic-home", ThreadID: "root-a", Session: "Codex root-a", Body: "synthetic question", For: "hq"}
	if _, fresh, err := Put(r); err != nil || !fresh {
		t.Fatal("thread-only request lost", fresh, err)
	}
	changed := r
	changed.ThreadID = "root-b"
	if _, _, err := Put(changed); !errors.Is(err, ErrConflict) {
		t.Fatal("same request retargeted", err)
	}
	if _, err := Claim(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(r.ID, "synthetic reply"); err != nil {
		t.Fatal(err)
	}
	if err := BeginReplyAttempt(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := BeginReplyAttempt(r.ID); !errors.Is(err, ErrState) {
		t.Fatal("uncertain reply automatically replayed", err)
	}
	if err := ClearReplyAttempt(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := BeginReplyAttempt(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := MarkDelivered(r.ID); err != nil {
		t.Fatal(err)
	}
	saved, _ := Get(r.ID)
	if !saved.Delivered || saved.ReplyAttempt != "accepted" || saved.ThreadID != "root-a" {
		t.Fatal("reply receipt lost exact source", saved)
	}
}
