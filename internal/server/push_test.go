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

// A device's APNs env (sandbox for dev builds, production for App Store) rides on
// every intent so ONE relay can route to the right APNs endpoint per token.
func TestPushEnvForwarded(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "Mac", nil)
	pm.Register(DeviceToken{Token: "prod-tok", Platform: "ios", Env: "production"})
	pm.Register(DeviceToken{Token: "dev-tok", Platform: "ios", Env: "sandbox"})

	pm.dispatch(Alert{Pane: "%1", Kind: "waiting", Agent: "Codex"})
	pm.pushBadge(1)
	byTok := map[string]string{}
	for _, in := range relay.intents() {
		byTok[in.Token] = in.Env // both the alert and the silent-badge intent per token
	}
	if byTok["prod-tok"] != "production" || byTok["dev-tok"] != "sandbox" {
		t.Fatalf("env not forwarded per token: %+v", byTok)
	}

	// Live Activity tokens carry their env too.
	relay2 := &fakeRelay{}
	pm2 := NewPushManager(relay2, nil, nil, "Mac", nil)
	pm2.RegisterActivity("act-prod", "production")
	pm2.PushLiveActivity(Tally{Waiting: 1})
	var got []PushIntent
	for i := 0; i < 200; i++ {
		if got = relay2.intents(); len(got) >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(got) != 1 || got[0].Env != "production" || !got[0].LiveActivity {
		t.Fatalf("activity intent env = %+v", got)
	}
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

func TestPushManagerUnregister(t *testing.T) {
	relay := &fakeRelay{}
	var saves int
	pm := NewPushManager(relay, nil, func([]DeviceToken) { saves++ }, "Mac", nil)
	pm.Register(DeviceToken{Token: "tok-a", Platform: "ios"})
	pm.Register(DeviceToken{Token: "tok-b", Platform: "ios"})
	saves = 0 // count only the unregister-driven persists below

	// Removing a server drops exactly that device's token and persists once.
	pm.Unregister("tok-a")
	if got := len(pm.Tokens()); got != 1 {
		t.Fatalf("tokens after unregister = %d, want 1", got)
	}
	if saves != 1 {
		t.Fatalf("save called %d times on a real unregister, want 1", saves)
	}

	// The dropped token no longer receives alerts or silent-badge pushes.
	pm.dispatch(Alert{Pane: "%1", Kind: "waiting", Agent: "Codex"})
	pm.pushBadge(1)
	for _, in := range relay.intents() {
		if in.Token == "tok-a" {
			t.Fatalf("unregistered token still pushed: %+v", in)
		}
	}

	// Idempotent: unknown token and empty token are no-ops (no persist).
	pm.Unregister("tok-a") // already gone
	pm.Unregister("")
	if saves != 1 {
		t.Fatalf("save called %d times, want 1 (no-op unregisters must not persist)", saves)
	}
	if got := len(pm.Tokens()); got != 1 {
		t.Fatalf("tokens = %d, want 1 (tok-b remains)", got)
	}
}

func TestPushManagerUnregisterActivity(t *testing.T) {
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "Mac", nil)
	pm.RegisterActivity("act-tok", "production")

	// Removing the server pushes an END for the held activity token (so a card this
	// Mac was keeping alive on the lock screen disappears), carrying its env.
	pm.UnregisterActivity("act-tok")
	var got []PushIntent
	for i := 0; i < 200; i++ {
		if got = relay.intents(); len(got) >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(got) != 1 || got[0].Token != "act-tok" || got[0].Event != "end" ||
		!got[0].LiveActivity || got[0].Env != "production" {
		t.Fatalf("end intent = %+v", got)
	}

	// The dropped token no longer receives tally UPDATES.
	pm.PushLiveActivity(Tally{Waiting: 1})
	time.Sleep(30 * time.Millisecond)
	for _, in := range relay.intents() {
		if in.Token == "act-tok" && in.Event == "update" {
			t.Fatalf("unregistered activity token still got a tally update: %+v", in)
		}
	}

	// Unknown / empty tokens are no-ops (no END pushed).
	before := len(relay.intents())
	pm.UnregisterActivity("nope")
	pm.UnregisterActivity("")
	time.Sleep(10 * time.Millisecond)
	if after := len(relay.intents()); after != before {
		t.Fatalf("no-op unregister pushed intents: %d -> %d", before, after)
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

	pm.RegisterActivity("act-tok-1", "production")
	pm.RegisterActivity("act-tok-2", "sandbox")
	pm.RegisterActivity("", "") // ignored

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

// --- push-token-device-binding ---

// UnregisterByDevice drops EVERY token bound to a device id (a device may register
// more than once across reinstalls), persists once, and treats an empty id as a no-op
// so a legacy revoke can't blanket-drop the UNLINKED tokens.
func TestUnregisterByDevice(t *testing.T) {
	saves := 0
	pm := NewPushManager(nil, nil, func([]DeviceToken) { saves++ }, "Mac", nil)
	pm.Register(DeviceToken{Token: "t1", DeviceID: "dev-A"})
	pm.Register(DeviceToken{Token: "t2", DeviceID: "dev-A"}) // same device, reinstalled
	pm.Register(DeviceToken{Token: "t3", DeviceID: "dev-B"})
	pm.Register(DeviceToken{Token: "legacy"}) // unlinked (empty id)
	saves = 0

	if n := pm.UnregisterByDevice("dev-A"); n != 2 {
		t.Fatalf("removed %d, want 2 (both of dev-A's tokens)", n)
	}
	if got := len(pm.Tokens()); got != 2 {
		t.Fatalf("remaining %d, want 2 (dev-B + legacy)", got)
	}
	if saves != 1 {
		t.Fatalf("persisted %d times, want 1", saves)
	}
	saves = 0
	if n := pm.UnregisterByDevice(""); n != 0 || saves != 0 {
		t.Fatalf("empty-id removed %d / saves %d, want 0/0 (must not touch unlinked)", n, saves)
	}
}

// Forget's selectors: orphans drops only unlinked tokens; a deviceId drops that
// device's; all drops everything.
func TestPushForgetSelectors(t *testing.T) {
	mk := func() *PushManager {
		pm := NewPushManager(nil, nil, nil, "Mac", nil)
		pm.Register(DeviceToken{Token: "a", DeviceID: "dev-A"})
		pm.Register(DeviceToken{Token: "b", DeviceID: "dev-B"})
		pm.Register(DeviceToken{Token: "leg1"})
		pm.Register(DeviceToken{Token: "leg2"})
		return pm
	}
	pm := mk()
	if n := pm.Forget("", true, false); n != 2 || len(pm.Tokens()) != 2 {
		t.Fatalf("orphans removed %d / remaining %d, want 2/2", n, len(pm.Tokens()))
	}
	pm = mk()
	if n := pm.Forget("dev-A", false, false); n != 1 || len(pm.Tokens()) != 3 {
		t.Fatalf("deviceId removed %d / remaining %d, want 1/3", n, len(pm.Tokens()))
	}
	pm = mk()
	if n := pm.Forget("", false, true); n != 4 || len(pm.Tokens()) != 0 {
		t.Fatalf("all removed %d / remaining %d, want 4/0", n, len(pm.Tokens()))
	}
}

func TestRedactToken(t *testing.T) {
	full := "0123456789abcdef0123456789abcdef"
	r := redactToken(full)
	if r == full {
		t.Fatal("redactToken returned the FULL token (must never leak it)")
	}
	prefix := strings.TrimSuffix(r, "…")
	if !strings.HasPrefix(full, prefix) || len(prefix) > 12 {
		t.Fatalf("redacted %q must be a short prefix of the token", r)
	}
}

// bindingServer wires Enroll + Push so the handler-level binding + master-only gates
// can be exercised with real scopes.
func bindingServer(t *testing.T) (h http.Handler, push *PushManager, devTok, devID, guestTok string) {
	t.Helper()
	enroll := NewEnrollManager(nil, nil)
	push = NewPushManager(nil, nil, nil, "Mac", nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:     enroll,
		Push:       push,
		AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
	})
	dev, _ := enroll.Redeem(enroll.Mint(), "phone")
	g := enroll.MintGuest("alice", []string{"%1"}, nil, 0)
	return s.Handler(), push, dev.Token, dev.ID, g.Token
}

// Register binds the token to the CALLER's enrolled device (from the bearer token,
// not the body), and revoking that device drops the token.
func TestPushRegisterBindsAndRevokeDrops(t *testing.T) {
	h, push, devTok, devID, _ := bindingServer(t)

	if rr := post(t, h, "/api/push/register", devTok, `{"token":"apns-XYZ","platform":"ios"}`); rr.Code != http.StatusOK {
		t.Fatalf("register = %d, want 200", rr.Code)
	}
	toks := push.Tokens()
	if len(toks) != 1 || toks[0].DeviceID != devID {
		t.Fatalf("token not bound to the caller's device: %+v (want deviceId %q)", toks, devID)
	}
	// Revoking the device drops its push token.
	if rr := post(t, h, "/api/devices/revoke", testToken, `{"id":"`+devID+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("revoke = %d, want 200", rr.Code)
	}
	if got := len(push.Tokens()); got != 0 {
		t.Fatalf("revoke left %d push token(s), want 0", got)
	}
}

// The token store is MASTER-only: a device or guest is refused on tokens + forget.
func TestPushTokensForgetMasterOnly(t *testing.T) {
	h, _, devTok, _, guestTok := bindingServer(t)

	if rr := do(t, h, http.MethodGet, "/api/push/tokens", testToken); rr.Code != http.StatusOK {
		t.Fatalf("master GET /api/push/tokens = %d, want 200", rr.Code)
	}
	for _, tok := range []string{devTok, guestTok} {
		if rr := do(t, h, http.MethodGet, "/api/push/tokens", tok); rr.Code != http.StatusForbidden {
			t.Fatalf("non-master GET /api/push/tokens = %d, want 403", rr.Code)
		}
		if rr := post(t, h, "/api/push/forget", tok, `{"all":true}`); rr.Code != http.StatusForbidden {
			t.Fatalf("non-master POST /api/push/forget = %d, want 403", rr.Code)
		}
	}
}

// GET /api/push/tokens never returns the full token — only a redacted prefix.
func TestPushTokensRedacted(t *testing.T) {
	h, _, devTok, _, _ := bindingServer(t)
	_ = post(t, h, "/api/push/register", devTok, `{"token":"apns-SECRET-FULL-TOKEN-VALUE","platform":"ios"}`)
	rr := do(t, h, http.MethodGet, "/api/push/tokens", testToken)
	if strings.Contains(rr.Body.String(), "apns-SECRET-FULL-TOKEN-VALUE") {
		t.Fatalf("GET /api/push/tokens leaked the full token: %s", rr.Body.String())
	}
}
