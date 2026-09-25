package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GET /api/tasks (chat-background-tasks): the dispatch ledger joined with each task's
// live pane status. Owner-only — a share link invites someone to watch a pane, and the
// ledger says everything this machine was told to do.

func taskServer(t *testing.T, list []TaskInfo, fail error) *Server {
	t.Helper()
	return New(Config{Addr: "x", Token: "master"}, Deps{
		Tasks: func() ([]TaskInfo, error) {
			if fail != nil {
				return nil, fail
			}
			return list, nil
		},
	})
}

func getTasks(t *testing.T, s *Server, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestTasksReportsTheLedgerWithLiveStatus(t *testing.T) {
	s := taskServer(t, []TaskInfo{
		{ID: "t1", Goal: "add --check to restore", Agent: "claude", Pane: "%21", Status: "waiting", Since: 1000},
		{ID: "t2", Goal: "weight the KB search", Agent: "codex", Pane: "%33", Status: "working", Since: 1100},
		{ID: "t3", Goal: "raise the nginx limit", Agent: "claude", Pane: "%19", Status: "idle", Since: 900},
	}, nil)
	rec := getTasks(t, s, "master")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got struct {
		Tasks []TaskInfo `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(got.Tasks))
	}
	if got.Tasks[0].Status != "waiting" || got.Tasks[0].Pane != "%21" {
		t.Errorf("the waiting task lost its state: %+v", got.Tasks[0])
	}
}

// A task whose pane is gone is still reported. It happened, and a list that silently
// drops it reads as though it never did.
func TestATaskWhosePaneIsGoneIsStillReported(t *testing.T) {
	s := taskServer(t, []TaskInfo{
		{ID: "t9", Goal: "check the cache", Agent: "codex", Pane: "%44", Status: "gone", Since: 500},
	}, nil)
	var got struct {
		Tasks []TaskInfo `json:"tasks"`
	}
	json.Unmarshal(getTasks(t, s, "master").Body.Bytes(), &got)
	if len(got.Tasks) != 1 || got.Tasks[0].Status != "gone" {
		t.Fatalf("a gone task was lost: %+v", got.Tasks)
	}
}

func TestAGuestDoesNotSeeTheDispatchLedger(t *testing.T) {
	s := taskServer(t, []TaskInfo{{ID: "t1", Goal: "secret work", Status: "working"}}, nil)
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("a share link", []string{"%1"}, nil, 0)

	rec := getTasks(t, s, guest.Token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a guest got %d, want 403", rec.Code)
	}
	if body := rec.Body.String(); len(body) > 0 && contains(body, "secret work") {
		t.Error("the refusal leaked a goal")
	}
}

func TestTasksSaysSoWhenItCannotRead(t *testing.T) {
	s := taskServer(t, nil, fmt.Errorf("ledger unreadable"))
	if rec := getTasks(t, s, "master"); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	// No dependency at all is a different answer from a broken one.
	bare := New(Config{Addr: "x", Token: "master"}, Deps{})
	if rec := getTasks(t, bare, "master"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestTasksRefusesAPost(t *testing.T) {
	s := taskServer(t, nil, nil)
	r := httptest.NewRequest(http.MethodPost, "/api/tasks", nil)
	r.Header.Set("Authorization", "Bearer master")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (hay == needle || len(needle) == 0 || indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
