package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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
	pm := NewPushManager(relay, nil, func(d []DeviceToken) { saved = append(saved, d) }, "MacBook Pro", nil)

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
		if in.Subtitle != "MacBook Pro" {
			t.Fatalf("subtitle = %q, want the server name 'MacBook Pro'", in.Subtitle)
		}
	}
}

func TestPushLiveActivity(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "", nil)

	// No activity registered → no push.
	pm.PushLiveActivity(Tally{Waiting: 1})
	if got := len(relay.intents()); got != 0 {
		t.Fatalf("intents with no activity = %d, want 0", got)
	}

	pm.RegisterActivity("act-tok-1")
	pm.RegisterActivity("act-tok-2")
	pm.RegisterActivity("") // ignored

	pm.PushLiveActivity(Tally{Waiting: 2, Working: 1, Idle: 3, WaitingTitle: "refactor api"})

	// Dispatch is async; poll until both activity tokens are pushed.
	var got []PushIntent
	for i := 0; i < 200; i++ {
		got = relay.intents()
		if len(got) >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(got) != 2 {
		t.Fatalf("intents = %d, want 2 (one per activity token)", len(got))
	}
	for _, in := range got {
		if !in.LiveActivity || in.Event != "update" {
			t.Fatalf("intent not a LA update: %+v", in)
		}
		if in.ContentState["waiting"] != 2 || in.ContentState["working"] != 1 ||
			in.ContentState["idle"] != 3 || in.ContentState["waitingTitle"] != "refactor api" {
			t.Fatalf("content-state = %+v", in.ContentState)
		}
		if in.Token != "act-tok-1" && in.Token != "act-tok-2" {
			t.Fatalf("unexpected token %q", in.Token)
		}
	}
}

// TestWaitingAlertCollapses: a waiting push carries the pane as its collapse-id so
// a re-nudge replaces the prior banner instead of stacking a second one.
func TestWaitingAlertCollapses(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, []DeviceToken{{Token: "t"}}, nil, "", nil)
	pm.dispatch(Alert{Pane: "%7", Kind: "waiting", Agent: "Claude"})
	got := relay.intents()
	if len(got) != 1 || got[0].CollapseID != "%7" {
		t.Fatalf("waiting intent collapse-id = %+v, want %q", got, "%7")
	}
}

// TestOnTallyBadgeSync: every tally change fans a silent, absolute-badge push to
// all devices (so a second/offline phone syncs its red dot to the waiting count).
func TestOnTallyBadgeSync(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, []DeviceToken{{Token: "a"}, {Token: "b"}}, nil, "", nil)
	pm.OnTally(Tally{Waiting: 2, Working: 1})

	silent := 0
	for _, in := range relay.intents() {
		if !in.Silent {
			continue
		}
		silent++
		if in.Badge == nil || *in.Badge != 2 {
			t.Errorf("badge = %v, want 2", in.Badge)
		}
		if in.Title != "" || in.Body != "" {
			t.Errorf("silent badge push must carry no alert copy: %+v", in)
		}
	}
	if silent != 2 {
		t.Fatalf("silent badge pushes = %d, want 2 (one per device)", silent)
	}
}

func TestPushManagerFormatter(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, []DeviceToken{{Token: "t"}}, nil, "",
		func(a Alert) (string, string) { return "T:" + a.Kind, "B:" + a.Agent })
	pm.dispatch(Alert{Kind: "done", Agent: "Claude"})
	got := relay.intents()
	if len(got) != 1 || got[0].Title != "T:done" || got[0].Body != "B:Claude" {
		t.Fatalf("formatter intent = %+v", got)
	}
}

func TestHandleRegister(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "", nil)
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
