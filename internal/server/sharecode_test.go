package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A share link and its code are one artifact: the code is minted with the link, lasts as
// long as the link does, and opens the same access however many times it is used.

func guestManager(t *testing.T, now *time.Time) (*EnrollManager, EnrolledDevice) {
	t.Helper()
	m := NewEnrollManager(nil, nil)
	m.now = func() time.Time { return *now }
	d := m.MintGuest("review with Lin", []string{"%12"}, []string{"%12"}, 0)
	return m, d
}

func TestALinkIsMintedWithItsCode(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)
	if link.Code == "" {
		t.Fatal("a link came out with no code")
	}
	got, ok := m.ShareCode(link.ID)
	if !ok || got != link.Code {
		t.Fatalf("ShareCode = %q %v, want the link's own %q", got, ok, link.Code)
	}
}

func TestTheCodeOpensTheLinkAndKeepsWorking(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)

	// Read out and typed back: lower case, the dash forgotten, an O for a zero.
	typed := strings.ToLower(strings.ReplaceAll(link.Code, "-", ""))
	typed = strings.ReplaceAll(typed, "0", "o")

	for i := 0; i < 3; i++ {
		got, why := m.RedeemWhy(typed, "browser")
		if why != "" {
			t.Fatalf("use %d refused: %s", i+1, why)
		}
		if got.Token != link.Token || got.ID != link.ID {
			t.Fatalf("use %d redeemed something else: %+v", i+1, got)
		}
	}
	// Nothing was created along the way.
	if n := len(m.Devices()); n != 1 {
		t.Errorf("roster grew to %d; a code must create nothing", n)
	}
	// A year later it still opens: a code has no life of its own.
	now = now.Add(365 * 24 * time.Hour)
	if _, why := m.RedeemWhy(link.Code, "browser"); why != "" {
		t.Errorf("a year on, the code was refused: %s", why)
	}
}

func TestTheLinkEndsTheCode(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)
	if !m.Revoke(link.ID) {
		t.Fatal("the link could not be revoked")
	}
	if _, why := m.RedeemWhy(link.Code, "browser"); why == "" {
		t.Error("a revoked link's code still handed over a token")
	}

	// An expiry does the same, and says so rather than pretending it never existed.
	past := m.MintGuest("yesterday", []string{"%12"}, nil, now.Add(-time.Hour).Unix())
	if _, why := m.RedeemWhy(past.Code, "browser"); why != RedeemExpired {
		t.Errorf("an expired link said %q, want %q", why, RedeemExpired)
	}
	if _, ok := m.ShareCode(past.ID); ok {
		t.Error("an expired link handed out its code")
	}
}

func TestOwnerDevicesHaveNoCode(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, _ := guestManager(t, &now)
	owner, _ := m.Redeem(m.Mint(), "iPhone")
	if owner.Code != "" {
		t.Error("an owner device was given a share code")
	}
	if _, ok := m.ShareCode(owner.ID); ok {
		t.Error("ShareCode answered for an owner device")
	}
	// And an owner pairing code still pairs an owner device.
	d, why := m.RedeemWhy(m.Mint(), "browser")
	if why != "" || d.Scope == "guest" {
		t.Fatalf("an owner code no longer pairs an owner device: %q %+v", why, d)
	}
}

// A link minted before codes existed gets one the first time it is asked for, so an old
// roster can be handed over the same way as a new one.
func TestAnOlderLinkGetsACodeWhenAsked(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, _ := guestManager(t, &now)
	old := EnrolledDevice{ID: "old1", Name: "before", Token: "tok-old", Scope: "guest", ScopeSet: true}
	m.mu.Lock()
	m.devices[old.Token] = old
	m.mu.Unlock()

	code, ok := m.ShareCode("old1")
	if !ok || code == "" {
		t.Fatalf("no code for an older link: %q %v", code, ok)
	}
	if d, why := m.RedeemWhy(code, "browser"); why != "" || d.Token != "tok-old" {
		t.Errorf("the new code did not open the older link: %q %+v", why, d)
	}
}

func TestShareCodeShape(t *testing.T) {
	code := newShareCode()
	if len(code) != shareCodeLen+1 || code[shareCodeLen/2] != '-' {
		t.Fatalf("unexpected shape: %q", code)
	}
	for _, r := range strings.ReplaceAll(code, "-", "") {
		if !strings.ContainsRune(shareCodeAlphabet, r) {
			t.Fatalf("%q is not in the alphabet a person can transcribe: %q", r, code)
		}
	}
	if strings.ContainsAny(code, "ILOU") {
		t.Errorf("code holds an ambiguous character: %q", code)
	}
	// Eight characters is the length the limiter can defend for a code that LASTS; six
	// was what a ten-minute code could afford. Shortening this is not a one-constant
	// change, so the floor is pinned here.
	if shareCodeLen < 8 {
		t.Errorf("shareCodeLen = %d: a lasting code needs at least 40 bits", shareCodeLen)
	}
}

func TestShareCodeEndToEndOverHTTP(t *testing.T) {
	em := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: em})
	link := em.MintGuest("review with Lin", []string{"%12"}, []string{"%12"}, 0)
	h := s.Handler()

	post := func(path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := post("/api/share/code", `{"id":"`+link.ID+`"}`, link.Token); rr.Code != http.StatusForbidden {
		t.Fatalf("a guest reading the code = %d, want 403", rr.Code)
	}
	rr := post("/api/share/code", `{"id":"`+link.ID+`"}`, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("the owner reading the code = %d (%s)", rr.Code, rr.Body.String())
	}
	var out struct{ Code string }
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out.Code != link.Code {
		t.Fatalf("got %q, want the link's own %q", out.Code, link.Code)
	}

	typed := strings.ToLower(strings.ReplaceAll(out.Code, "-", ""))
	rr = post("/api/enroll", `{"enrollCode":"`+typed+`","name":"browser"}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("redeeming = %d (%s)", rr.Code, rr.Body.String())
	}
	var got struct{ Token string }
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Token != link.Token {
		t.Error("redeemed a different token than the link's")
	}
	// And again, because a code lasts.
	if rr := post("/api/enroll", `{"enrollCode":"`+typed+`"}`, ""); rr.Code != http.StatusOK {
		t.Errorf("a second use = %d, want 200", rr.Code)
	}
}

// Eight characters is only defensible while guessing is bounded, so the two are tested
// together.
func TestGuessingIsStopped(t *testing.T) {
	em := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: em})
	link := em.MintGuest("probe", nil, nil, 0)
	h := s.Handler()

	try := func(guess, from string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/enroll", strings.NewReader(`{"enrollCode":"`+guess+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = from + ":51000"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	var last int
	for i := 0; i < 12; i++ {
		last = try("ZZZZZZZZ", "203.0.113.9")
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("a run of wrong codes ended in %d, want 429", last)
	}
	if got := try(link.Code, "198.51.100.4"); got != http.StatusOK {
		t.Errorf("a good code from another address = %d, want 200", got)
	}
}

func TestASuccessCostsNothing(t *testing.T) {
	l := newRedeemLimiter()
	for i := 0; i < l.perIP-1; i++ {
		l.failed("203.0.113.9")
	}
	if !l.allow("203.0.113.9") {
		t.Fatal("locked out one failure early")
	}
	l.failed("203.0.113.9")
	if l.allow("203.0.113.9") {
		t.Error("the line was not held")
	}
}

func TestTheLockoutEnds(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newRedeemLimiter()
	l.now = func() time.Time { return now }
	for i := 0; i < l.perIP; i++ {
		l.failed("203.0.113.9")
	}
	if l.allow("203.0.113.9") {
		t.Fatal("not limited when it should be")
	}
	now = now.Add(l.window + time.Second)
	if !l.allow("203.0.113.9") {
		t.Error("still locked out after the window passed")
	}
}
