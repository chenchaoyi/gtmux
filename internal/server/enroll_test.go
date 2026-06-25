package server

import (
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
