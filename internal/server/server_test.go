package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "secret-token"

// newTestServer wires a Server with controllable fake deps.
func newTestServer() (*Server, *fakeDeps) {
	f := &fakeDeps{agents: []byte(`[{"pane_id":"%1"}]`)}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		AgentsJSON: f.AgentsJSON,
		PaneText:   f.PaneText,
		Focus:      f.Focus,
	})
	return s, f
}

type fakeDeps struct {
	agents     []byte
	agentsErr  error
	paneText   string
	paneOK     bool
	focusErr   error
	focusCalls []string
}

func (f *fakeDeps) AgentsJSON() ([]byte, error)       { return f.agents, f.agentsErr }
func (f *fakeDeps) PaneText(id string) (string, bool) { return f.paneText, f.paneOK }
func (f *fakeDeps) Focus(id string) error             { f.focusCalls = append(f.focusCalls, id); return f.focusErr }

// TestSendReturnsPaneSnapshot: POST /api/send echoes the post-send pane text +
// cursor so the client renders the echo in one round-trip (no separate /api/pane).
func TestSendReturnsPaneSnapshot(t *testing.T) {
	old := sendSettle
	sendSettle = 0 // don't sleep in the test
	defer func() { sendSettle = old }()

	var sent []string
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Send: func(id, text, key string, enter bool) error {
			sent = append(sent, id+"|"+text)
			return nil
		},
		PaneText:   func(id string) (string, bool) { return "$ ls\nfile.txt", true },
		PaneCursor: func(id string) (x, up int, visible, ok bool) { return 4, 0, true, true },
	})

	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`{"id":"%1","text":"ls","enter":true}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("send = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Text   string `json:"text"`
		Cursor *struct {
			X       int  `json:"x"`
			Up      int  `json:"up"`
			Visible bool `json:"visible"`
		} `json:"cursor"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("send body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	if body.Text != "$ ls\nfile.txt" {
		t.Errorf("text = %q, want the post-send pane snapshot", body.Text)
	}
	if body.Cursor == nil || body.Cursor.X != 4 || !body.Cursor.Visible {
		t.Errorf("cursor = %+v, want {x:4 visible:true}", body.Cursor)
	}
	if len(sent) != 1 || sent[0] != "%1|ls" {
		t.Errorf("Send calls = %v, want [%%1|ls]", sent)
	}
}

func do(t *testing.T, h http.Handler, method, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHealthNoAuth(t *testing.T) {
	s, _ := newTestServer()
	rr := do(t, s.Handler(), http.MethodGet, "/api/health", "") // no token
	if rr.Code != http.StatusOK {
		t.Fatalf("health = %d, want 200", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("health status = %q, want ok", body["status"])
	}
}

func TestAgentsAuth(t *testing.T) {
	s, f := newTestServer()
	h := s.Handler()

	if rr := do(t, h, http.MethodGet, "/api/agents", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/agents", "wrong"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", rr.Code)
	}

	rr := do(t, h, http.MethodGet, "/api/agents", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("good token = %d, want 200", rr.Code)
	}
	if got := rr.Body.String(); got != string(f.agents) {
		t.Fatalf("agents body = %q, want %q", got, f.agents)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestAgentsError(t *testing.T) {
	s, f := newTestServer()
	f.agentsErr = errors.New("boom")
	rr := do(t, s.Handler(), http.MethodGet, "/api/agents", testToken)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("agents error = %d, want 500", rr.Code)
	}
}

func TestPane(t *testing.T) {
	s, f := newTestServer()
	h := s.Handler()

	if rr := do(t, h, http.MethodGet, "/api/pane", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing id = %d, want 400", rr.Code)
	}

	f.paneOK = false
	if rr := do(t, h, http.MethodGet, "/api/pane?id=%2599", testToken); rr.Code != http.StatusNotFound {
		t.Fatalf("gone pane = %d, want 404", rr.Code)
	}

	f.paneOK = true
	f.paneText = "hello\nworld"
	rr := do(t, h, http.MethodGet, "/api/pane?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("pane = %d, want 200", rr.Code)
	}
	var pr paneResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &pr); err != nil {
		t.Fatalf("pane body: %v", err)
	}
	if pr.ID != "%1" || pr.Text != "hello\nworld" {
		t.Fatalf("pane response = %+v", pr)
	}
}

// TestPaneCursor wires PaneCursor and asserts the bottom-anchored cursor JSON
// shape (the cursor branch of handlePane is otherwise never exercised).
func TestPaneCursor(t *testing.T) {
	f := &fakeDeps{paneOK: true, paneText: "❯ "}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		PaneText:   f.PaneText,
		PaneCursor: func(id string) (x, up int, visible, ok bool) { return 4, 0, true, true },
	})
	rr := do(t, s.Handler(), http.MethodGet, "/api/pane?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("pane = %d, want 200", rr.Code)
	}
	var pr paneResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &pr); err != nil {
		t.Fatalf("pane body: %v", err)
	}
	if pr.Cursor == nil {
		t.Fatalf("cursor missing; body=%s", rr.Body.String())
	}
	if pr.Cursor.X != 4 || pr.Cursor.Up != 0 || !pr.Cursor.Visible {
		t.Fatalf("cursor = %+v, want {4 0 true}", *pr.Cursor)
	}

	// Cursor unresolved (ok=false) → field omitted.
	s2 := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		PaneText:   f.PaneText,
		PaneCursor: func(id string) (x, up int, visible, ok bool) { return 0, 0, false, false },
	})
	rr2 := do(t, s2.Handler(), http.MethodGet, "/api/pane?id=%251", testToken)
	if strings.Contains(rr2.Body.String(), "\"cursor\"") {
		t.Fatalf("cursor should be omitted when unresolved; body=%s", rr2.Body.String())
	}
}

// TestTranscript covers the /api/transcript handler: 503 (no dep), 400 (no id),
// 404 (parse error), 200 passthrough.
func TestTranscript(t *testing.T) {
	// No Transcript dep wired → 503.
	h0 := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{}).Handler()
	if rr := do(t, h0, http.MethodGet, "/api/transcript?id=%251", testToken); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("no Transcript dep = %d, want 503", rr.Code)
	}

	turns := []byte(`[{"prompt":"hi","response":"yo","segments":[{"text":"yo"}],"time":"2026-06-29T10:00:00Z"}]`)
	h := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Transcript: func(id string) ([]byte, int, error) {
			if id != "%1" {
				return nil, 0, errors.New("transcript failed")
			}
			return turns, 0, nil
		},
	}).Handler()

	if rr := do(t, h, http.MethodGet, "/api/transcript", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing id = %d, want 400", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/transcript?id=%251", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/transcript?id=%2599", testToken); rr.Code != http.StatusNotFound {
		t.Fatalf("parse error = %d, want 404", rr.Code)
	}
	rr := do(t, h, http.MethodGet, "/api/transcript?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("ok = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if got := rr.Body.String(); got != string(turns) {
		t.Fatalf("transcript body = %q, want passthrough %q", got, turns)
	}
}

func TestFocus(t *testing.T) {
	s, f := newTestServer()
	h := s.Handler()

	if rr := do(t, h, http.MethodGet, "/api/focus?id=%251", testToken); rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET focus = %d, want 405", rr.Code)
	}
	if rr := do(t, h, http.MethodPost, "/api/focus", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("focus no id = %d, want 400", rr.Code)
	}

	rr := do(t, h, http.MethodPost, "/api/focus?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("focus = %d, want 200", rr.Code)
	}
	if len(f.focusCalls) != 1 || f.focusCalls[0] != "%1" {
		t.Fatalf("focus calls = %v, want [%%1]", f.focusCalls)
	}

	f.focusErr = errors.New("pane gone")
	if rr := do(t, h, http.MethodPost, "/api/focus?id=%252", testToken); rr.Code != http.StatusNotFound {
		t.Fatalf("focus error = %d, want 404", rr.Code)
	}
}

// TestFocusBodyIgnored guards that focus reads the id from the query, not body.
func TestFocusBodyIgnored(t *testing.T) {
	s, _ := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/focus?id=%251", strings.NewReader("ignored"))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		b, _ := io.ReadAll(rr.Body)
		t.Fatalf("focus = %d (%s), want 200", rr.Code, b)
	}
}

func TestDiff(t *testing.T) {
	// No Diff dep wired → 503.
	h0 := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{}).Handler()
	if rr := do(t, h0, http.MethodGet, "/api/diff?id=%251", testToken); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("no Diff dep = %d, want 503", rr.Code)
	}

	h := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Diff: func(id string) (string, error) {
			if id != "%1" {
				return "", errors.New("pane not found")
			}
			return "# branch main\ndiff --git a/x b/x\n+added\n", nil
		},
	}).Handler()

	if rr := do(t, h, http.MethodGet, "/api/diff", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing id = %d, want 400", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/diff?id=%251", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
	if rr := do(t, h, http.MethodGet, "/api/diff?id=%2599", testToken); rr.Code != http.StatusNotFound {
		t.Fatalf("diff error = %d, want 404", rr.Code)
	}
	rr := do(t, h, http.MethodGet, "/api/diff?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("ok diff = %d, want 200", rr.Code)
	}
	var resp struct{ ID, Diff string }
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "%1" || !strings.Contains(resp.Diff, "diff --git") {
		t.Fatalf("body = %+v", resp)
	}
}
