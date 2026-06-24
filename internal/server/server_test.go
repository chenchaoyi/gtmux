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
