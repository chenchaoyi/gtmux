package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestEnrollRedeemRoundTrip(t *testing.T) {
	var saved [][]EnrolledDevice
	m := NewEnrollManager(nil, func(d []EnrolledDevice) { saved = append(saved, d) })

	code := m.Mint()
	d, ok := m.Redeem(code, "  iPhone\x07  ") // control char + spaces get sanitized
	if !ok {
		t.Fatal("redeem valid code failed")
	}
	if d.Token == "" || d.ID == "" {
		t.Fatalf("device missing token/id: %+v", d)
	}
	if d.Name != "iPhone" {
		t.Errorf("name = %q, want sanitized 'iPhone'", d.Name)
	}
	if !m.ValidToken(d.Token) {
		t.Error("issued token should authenticate")
	}
	if len(saved) != 1 {
		t.Errorf("save called %d times, want 1 (on enroll)", len(saved))
	}

	// Single-use: the same code can't be redeemed twice.
	if _, ok := m.Redeem(code, "again"); ok {
		t.Error("code must be single-use")
	}
}

func TestEnrollCodeExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	m := NewEnrollManager(nil, nil)
	m.now = fixedClock(now)
	code := m.Mint()

	m.now = fixedClock(now.Add(enrollCodeTTL + time.Second)) // past expiry
	if _, ok := m.Redeem(code, "x"); ok {
		t.Error("expired code must not redeem")
	}
}

func TestEnrollRevoke(t *testing.T) {
	m := NewEnrollManager(nil, nil)
	d, _ := m.Redeem(m.Mint(), "p")
	if !m.ValidToken(d.Token) {
		t.Fatal("token should be valid before revoke")
	}
	if !m.Revoke(d.ID) {
		t.Error("revoke should report found")
	}
	if m.ValidToken(d.Token) {
		t.Error("revoked token must no longer authenticate")
	}
	if m.Revoke("nope") {
		t.Error("revoking an unknown id should report not-found")
	}
}

func TestEnrollSeedsRoster(t *testing.T) {
	m := NewEnrollManager([]EnrolledDevice{{ID: "a", Token: "tok-a", Name: "old"}}, nil)
	if !m.ValidToken("tok-a") {
		t.Error("persisted device should authenticate after restart")
	}
}

// TestAuthAcceptsDeviceToken: a request with an enrolled device token passes auth,
// alongside the master token; an unknown token is rejected.
func TestAuthAcceptsDeviceToken(t *testing.T) {
	en := NewEnrollManager(nil, nil)
	d, _ := en.Redeem(en.Mint(), "phone")
	s := New(Config{Addr: "x", Token: "master"}, Deps{
		Enroll:     en,
		AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
	})
	h := s.Handler()

	call := func(tok string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if call("master") != http.StatusOK {
		t.Error("master token should pass")
	}
	if call(d.Token) != http.StatusOK {
		t.Error("enrolled device token should pass")
	}
	if call("bogus") != http.StatusUnauthorized {
		t.Error("unknown token should be 401")
	}
}

// TestDeviceListAndRevoke: GET /api/devices lists without tokens; POST
// /api/devices/revoke drops one and its token stops authenticating immediately.
func TestDeviceListAndRevoke(t *testing.T) {
	en := NewEnrollManager(nil, nil)
	d, _ := en.Redeem(en.Mint(), "Ada's iPhone")
	s := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en})
	h := s.Handler()

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer master")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	list := do(http.MethodGet, "/api/devices", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list = %d", list.Code)
	}
	body := list.Body.String()
	if !strings.Contains(body, d.ID) || !strings.Contains(body, "Ada's iPhone") {
		t.Errorf("device not listed: %s", body)
	}
	if strings.Contains(body, d.Token) {
		t.Error("GET /api/devices must NOT leak tokens")
	}

	// revoke needs auth
	if do(http.MethodPost, "/api/devices/revoke", `{"id":"`+d.ID+`"}`).Code != http.StatusOK {
		t.Fatal("revoke should 200")
	}
	if en.ValidToken(d.Token) {
		t.Error("revoked device token must stop authenticating immediately")
	}
}

// TestEnrollEndpoints: mint (auth'd) then redeem (public) over HTTP.
func TestEnrollEndpoints(t *testing.T) {
	en := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en})
	h := s.Handler()

	// mint requires auth
	mint := func(tok string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/enroll/mint", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	if mint("").Code != http.StatusUnauthorized {
		t.Error("mint without auth should be 401")
	}
	body := mint("master").Body.String()
	if !strings.Contains(body, "enrollCode") {
		t.Fatalf("mint response missing code: %s", body)
	}

	// pull the code back out via the manager to redeem it over HTTP
	code := en.Mint()
	redeem := func(payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/enroll", strings.NewReader(payload))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	rr := redeem(`{"enrollCode":"` + code + `","name":"My Phone"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "token") {
		t.Fatalf("redeem = %d %s", rr.Code, rr.Body.String())
	}
	if redeem(`{"enrollCode":"wrong"}`).Code != http.StatusUnauthorized {
		t.Error("bad code should be 401")
	}
}

// The roster has an ORDER, and it comes from the data.
//
// It used to come from a Go map, so two calls a second apart returned different orders —
// what the commander read as "the menu bar and the app show them differently". Neither
// surface sorts; each was rendering a different shuffle of the same list.
func TestRosterOrderIsMostRecentlySeenFirst(t *testing.T) {
	// Keyed by token — a device with none is not in the roster at all.
	m := NewEnrollManager([]EnrolledDevice{
		{ID: "never", Token: "t1", Name: "never", EnrolledAt: 50},
		{ID: "old", Token: "t2", Name: "old", EnrolledAt: 10, LastSeen: 100},
		{ID: "recent", Token: "t3", Name: "recent", EnrolledAt: 20, LastSeen: 900},
	}, nil)
	var got []string
	for _, d := range m.Devices() {
		got = append(got, d.ID)
	}
	want := []string{"recent", "old", "never"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("order = %v, want %v (a device that never connected sorts last)", got, want)
		}
	}
	// Stability is the point: a list that reshuffles between polls is unreadable.
	for i := 0; i < 5; i++ {
		again := m.Devices()
		for j := range again {
			if again[j].ID != got[j] {
				t.Fatalf("order changed between calls: %v then %v", got, again)
			}
		}
	}
}

// A pairing window has to know when the code on its QR died. Codes live only in memory,
// so a serve restart drops every one of them, and nothing told the window: it kept
// showing a code the new serve had never issued, and the phone that scanned it was told
// the code had expired. The boot on /api/health and on the mint response is that signal.
func TestBootNamesTheLifetimeOfEveryCode(t *testing.T) {
	en := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en})
	h := s.Handler()

	get := func(method, path string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer master")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s = %d", method, path, rr.Code)
		}
		var out map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return out
	}

	first := get(http.MethodGet, "/api/health")["boot"]
	if first == nil || first == "" {
		t.Fatal("/api/health carries no boot")
	}
	if again := get(http.MethodGet, "/api/health")["boot"]; again != first {
		t.Errorf("boot changed between two probes of the same serve: %v then %v", first, again)
	}
	if minted := get(http.MethodPost, "/api/enroll/mint")["boot"]; minted != first {
		t.Errorf("the mint response names boot %v, the serve is %v", minted, first)
	}

	// A restart is a new manager: the codes are gone, and the boot says so.
	if NewEnrollManager(nil, nil).Boot() == en.Boot() {
		t.Error("a fresh manager reused the old boot; a window could not tell its code had died")
	}
}

// Without an enroll manager there are no codes, so there is no boot to report.
func TestHealthOmitsBootWithoutEnrollment(t *testing.T) {
	s := New(Config{Addr: "x", Token: "master"}, Deps{})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if strings.Contains(rr.Body.String(), "boot") {
		t.Errorf("health reports a boot with no code store behind it: %s", rr.Body.String())
	}
}

func TestRenamePairedDevicePreservesIdentityAndPersists(t *testing.T) {
	original := EnrolledDevice{ID: "a", Token: "token-a", Name: "iPhone", Platform: "iOS 26.6", LastIP: "10.0.0.2", EnrolledAt: 10, LastSeen: 20}
	var saved []EnrolledDevice
	m := NewEnrollManager([]EnrolledDevice{original}, func(rows []EnrolledDevice) { saved = rows })
	got, ok := m.RenameOwner("a", "  gtmux · 我的手机\x00  ")
	if !ok || got != "gtmux · 我的手机" {
		t.Fatalf("rename = %q, %v", got, ok)
	}
	d, ok := m.DeviceByToken(original.Token)
	if !ok || d.ID != original.ID || d.Scope != original.Scope || d.Platform != original.Platform || d.LastIP != original.LastIP || d.EnrolledAt != 10 || d.LastSeen != 20 || !d.NameIsCustom {
		t.Fatalf("identity changed: %+v", d)
	}
	reopened := NewEnrollManager(saved, nil)
	if after, ok := reopened.DeviceByToken(original.Token); !ok || after.Name != got || !after.NameIsCustom {
		t.Fatal("edited label did not survive restart")
	}
	m.SetClient(original.Token, "iOS 27.0", "10.0.0.3")
	if d, _ := m.DeviceByToken(original.Token); d.Name != got {
		t.Fatal("client refresh clobbered custom label")
	}
	if _, ok := m.RenameOwner("missing", "new"); ok {
		t.Fatal("renamed missing device")
	}
	if _, ok := m.RenameOwner("a", "\n\t"); ok {
		t.Fatal("accepted blank label")
	}
}

func TestRenameDeviceEndpointScopeAndValidation(t *testing.T) {
	en := NewEnrollManager([]EnrolledDevice{{ID: "d", Name: "iPhone", Token: "owner"}, {ID: "g", Name: "Guest", Token: "guest", Scope: "guest"}}, nil)
	h := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en}).Handler()
	for _, tc := range []struct {
		token, method, body string
		status              int
	}{
		{"", "POST", `{"id":"d","name":"ccy's iPhone"}`, 401},
		{"owner", "POST", `{"id":"d","name":"ccy's iPhone"}`, 403},
		{"guest", "POST", `{"id":"d","name":"ccy's iPhone"}`, 403},
		{"master", "GET", ``, 405},
		{"master", "POST", `{"id":"d","name":"  "}`, 400},
		{"master", "POST", `{"name":"label"}`, 400},
		{"master", "POST", `{"id":"missing","name":"label"}`, 404},
		{"master", "POST", `{"id":"g","name":"label"}`, 404},
		{"master", "POST", `{"id":"d","name":"ccy's iPhone"}`, 200},
	} {
		r := httptest.NewRequest(tc.method, "/api/devices/rename", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %s %s = %d: %s", tc.token, tc.method, tc.body, w.Code, w.Body.String())
		}
		if tc.status != 200 {
			if d, _ := en.DeviceByToken("owner"); d.Name != "iPhone" {
				t.Fatal("failed rename changed label")
			}
		}
		if strings.Contains(w.Body.String(), `"token"`) {
			t.Fatal("rename response exposes token")
		}
	}
}

func TestBrowserEnrollmentRecordsAvailableClientDetails(t *testing.T) {
	var saved []EnrolledDevice
	en := NewEnrollManager(nil, func(d []EnrolledDevice) { saved = d })
	code := en.Mint()
	h := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en}).Handler()
	r := httptest.NewRequest("POST", "/api/enroll", strings.NewReader(`{"enrollCode":"`+code+`","name":"Browser"}`))
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/141.0.0.0 Safari/537.36")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("enroll: %d", w.Code)
	}
	en.FlushIfDirty()
	if len(saved) != 1 || saved[0].Platform != "Chrome 141 · macOS" || saved[0].LastIP == "" {
		t.Fatalf("details not captured: %+v", saved)
	}
	// A later authenticated browser updates stale details without overwriting its label.
	r = httptest.NewRequest("GET", "/api/devices", nil)
	r.Header.Set("Authorization", "Bearer "+saved[0].Token)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) Version/18.0 Safari/605.1.15")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("reconnect: %d", w.Code)
	}
	en.FlushIfDirty()
	if saved[0].Platform != "Safari 18 · macOS" || saved[0].Name != "Browser" {
		t.Fatalf("refresh: %+v", saved)
	}
}

func TestDeviceNameLimitPreservesUTF8(t *testing.T) {
	name := sanitizeDeviceName(strings.Repeat("手机", 30))
	if !json.Valid([]byte(`"`+name+`"`)) || len([]rune(name)) != 40 {
		t.Fatalf("invalid/badly bounded name %q", name)
	}
	if got := cleanDeviceName("\u0085\n"); got != "" {
		t.Fatalf("control-only name accepted %q", got)
	}
}
