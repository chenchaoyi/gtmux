package hqworkspace

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRegisterExistingGoalDoesNotReplayWorkOrRewriteSources(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "notes"), 0700); err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(home, "notes", "board.md")
	const original = "# Board\nExisting investigation; waiting for a disk decision.\n"
	if err := os.WriteFile(board, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(home)
	d, err := s.Read()
	if err != nil || len(d.Matters) != 0 {
		t.Fatal("a read guessed matters from prose", d, err)
	}
	origin := Origin{Kind: "board", Ref: "notes/board.md#disk-decision"}
	m, err := s.Register("disk-review", "Disk investigation", "Awaiting approval; nothing deleted.", "needs-input", origin, 100)
	if err != nil || m.State != "needs-input" || m.Origin == nil || *m.Origin != origin || len(m.Participants) != 0 {
		t.Fatal(m, err)
	}
	d, err = s.Read()
	if err != nil || len(Matters(d)) != 1 || len(d.Messages) != 1 || len(d.Changes) != 1 {
		t.Fatal(d, err)
	}
	for _, msg := range d.Messages {
		if msg.Speaker != "hq" || msg.Kind != "report" || msg.Delivery != "" || msg.Body != m.Assessment {
			t.Fatal("registration invented a user request or lost the assessment", msg)
		}
	}
	if attempted, err := s.Pump(DeliveryIO{
		Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "hq-session"}, "" },
		Send:    func(Conversation, string) Delivery { t.Fatal("registration attempted delivery"); return Delivery{} },
	}, 101); err != nil || attempted {
		t.Fatal("registration queued execution", attempted, err)
	}
	if b, err := os.ReadFile(board); err != nil || string(b) != original {
		t.Fatal("registration changed the source board", err)
	}
}

func TestRegisterRetryAfterProgressPreservesCurrentMatterAndRejectsChangedIntent(t *testing.T) {
	s := New(t.TempDir())
	o := Origin{Kind: "task", Ref: "task-immutable-id"}
	if _, err := s.Register("goal-one", "Original", "Verified assignment", "open", o, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.Update("goal-one", "Renamed", "needs-input", "New assessment", 0, 101); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Read()
	m, err := s.Register("goal-one", "Original", "Verified assignment", "open", o, 102)
	if err != nil || m.Title != "Renamed" || m.State != "needs-input" || m.Assessment != "New assessment" {
		t.Fatal("retry reset subsequent progress", m, err)
	}
	for _, mutate := range []func(*Origin, *string, *string, *string){
		func(o *Origin, title, body, state *string) { o.Ref = "another-task" },
		func(o *Origin, title, body, state *string) { o.Kind = "board" },
		func(o *Origin, title, body, state *string) { *title = "Another goal" },
		func(o *Origin, title, body, state *string) { *body = "Changed intent" },
		func(o *Origin, title, body, state *string) { *state = "completed" },
	} {
		origin, title, body, state := o, "Original", "Verified assignment", "open"
		mutate(&origin, &title, &body, &state)
		if _, err := s.Register("goal-one", title, body, state, origin, 103); err != ErrConflict {
			t.Fatal("changed registration accepted", err)
		}
	}
	after, _ := s.Read()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("retry/conflict changed records")
	}
}

func TestRegistrationRejectsMissingEvidenceAndStorageFailure(t *testing.T) {
	for _, o := range []Origin{{}, {Kind: "board"}, {Kind: "pane", Ref: "%10"}, {Kind: "task", Ref: strings.Repeat("x", 2049)}} {
		s := New(t.TempDir())
		if _, err := s.Register("goal-one", "Goal", "Evidence", "open", o, 100); err == nil {
			t.Fatal("invalid origin accepted", o)
		}
		d, _ := s.Read()
		if len(d.Matters) != 0 || len(d.Messages) != 0 {
			t.Fatal("validation left partial records")
		}
	}
	o := Origin{Kind: "conversation", Ref: "codex/home/original-session#message-4"}
	if _, err := New(t.TempDir()).Register("goal-one", "Goal", "Evidence", "", o, 100); err == nil {
		t.Fatal("missing explicit state accepted")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "workspace"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(home).Register("goal-one", "Goal", "Evidence", "open", o, 100); err == nil {
		t.Fatal("storage failure reported success")
	}
}

func TestConcurrentRegistrationConverges(t *testing.T) {
	s := New(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Register("goal-one", "Goal", "Evidence", "open", Origin{Kind: "task", Ref: "task-one"}, 100); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	d, err := s.Read()
	if err != nil || d.Revision != 1 || len(d.Matters) != 1 || len(d.Messages) != 1 {
		t.Fatal("concurrent retry duplicated records", d, err)
	}
}
