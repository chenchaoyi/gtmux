package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeRelay records every intent it is asked to send.
type fakeRelay struct {
	mu   sync.Mutex
	sent []PushIntent
	err  error
}

func (f *fakeRelay) Send(i PushIntent) error {
	f.mu.Lock()
	f.sent = append(f.sent, i)
	f.mu.Unlock()
	return f.err
}

func (f *fakeRelay) intents() []PushIntent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]PushIntent(nil), f.sent...)
}

func TestPushManagerDispatch(t *testing.T) {
	relay := &fakeRelay{}
	var saved [][]DeviceToken
	pm := NewPushManager(relay, nil, func(d []DeviceToken) { saved = append(saved, d) }, nil)

	pm.Register(DeviceToken{Token: "tok-a", Platform: "ios"})
	pm.Register(DeviceToken{Token: "tok-b"}) // platform defaults to ios

	if got := len(pm.Tokens()); got != 2 {
		t.Fatalf("tokens = %d, want 2", got)
	}
	if len(saved) != 2 {
		t.Fatalf("save called %d times, want 2", len(saved))
	}
	for _, d := range pm.Tokens() {
		if d.Platform != "ios" {
			t.Fatalf("platform = %q, want ios (default)", d.Platform)
		}
	}

	// dispatch is the synchronous core OnAlert runs in a goroutine.
	pm.dispatch(Alert{Pane: "%3", Kind: "waiting", Agent: "Codex", Task: "build"})
	got := relay.intents()
	if len(got) != 2 {
		t.Fatalf("intents = %d, want 2 (one per token)", len(got))
	}
	for _, in := range got {
		if in.Pane != "%3" || in.Kind != "waiting" {
			t.Fatalf("intent pane/kind = %+v", in)
		}
		if !strings.Contains(in.Title, "Codex") || in.Body != "build" {
			t.Fatalf("fallback copy = %q / %q", in.Title, in.Body)
		}
	}
}

func TestPushManagerFormatter(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, []DeviceToken{{Token: "t"}}, nil,
		func(a Alert) (string, string) { return "T:" + a.Kind, "B:" + a.Agent })
	pm.dispatch(Alert{Kind: "done", Agent: "Claude"})
	got := relay.intents()
	if len(got) != 1 || got[0].Title != "T:done" || got[0].Body != "B:Claude" {
		t.Fatalf("formatter intent = %+v", got)
	}
}

func TestHandleRegister(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Push: pm})
	h := s.Handler()

	// GET → 405
	if rr := do(t, h, http.MethodGet, "/api/push/register", testToken); rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET register = %d, want 405", rr.Code)
	}
	// no token → 401 (auth applies)
	if rr := do(t, h, http.MethodPost, "/api/push/register", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no auth = %d, want 401", rr.Code)
	}

	// good POST stores the token
	req := httptest.NewRequest(http.MethodPost, "/api/push/register", strings.NewReader(`{"token":"abc","platform":"ios"}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("register = %d, want 200", rr.Code)
	}
	if toks := pm.Tokens(); len(toks) != 1 || toks[0].Token != "abc" {
		t.Fatalf("stored tokens = %+v", pm.Tokens())
	}

	// bad body → 400
	req = httptest.NewRequest(http.MethodPost, "/api/push/register", strings.NewReader(`{"platform":"ios"}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty token = %d, want 400", rr.Code)
	}
}

func TestHandleRegisterPushDisabled(t *testing.T) {
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{}) // no Push
	req := httptest.NewRequest(http.MethodPost, "/api/push/register", strings.NewReader(`{"token":"x"}`))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("push disabled = %d, want 503", rr.Code)
	}
}

func TestHTTPRelaySend(t *testing.T) {
	var gotBody PushIntent
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewHTTPRelay(ts.URL, "relay-secret")
	if err := r.Send(PushIntent{Token: "t", Title: "hi", Pane: "%1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotAuth != "Bearer relay-secret" {
		t.Fatalf("relay auth = %q", gotAuth)
	}
	if gotBody.Token != "t" || gotBody.Title != "hi" || gotBody.Pane != "%1" {
		t.Fatalf("relay body = %+v", gotBody)
	}

	// empty URL → no-op, no error
	if err := NewHTTPRelay("", "").Send(PushIntent{Token: "t"}); err != nil {
		t.Fatalf("empty url send: %v", err)
	}
}

func TestHTTPRelaySendNon2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()
	if err := NewHTTPRelay(ts.URL, "").Send(PushIntent{Token: "t"}); err == nil {
		t.Fatalf("non-2xx should error")
	}
}
