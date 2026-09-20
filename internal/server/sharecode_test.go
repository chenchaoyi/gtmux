package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A guest link handed over as a code that a person can read out loud and type.

func guestManager(t *testing.T, now *time.Time) (*EnrollManager, EnrolledDevice) {
	t.Helper()
	m := NewEnrollManager(nil, nil)
	m.now = func() time.Time { return *now }
	d := m.MintGuest("review with Lin", []string{"%12"}, []string{"%12"}, 0)
	return m, d
}

func TestShareCodeRedeemsToTheLinkItWasMintedFor(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)

	code, ok := m.MintShareCode(link.ID)
	if !ok {
		t.Fatal("no code minted for a live link")
	}
	// It is read out loud and typed back in: lower case, the dash forgotten, an O for a
	// zero. All of that has to land on the same code.
	typed := strings.ToLower(strings.ReplaceAll(code, "-", ""))
	typed = strings.ReplaceAll(typed, "0", "o")
	got, why := m.RedeemWhy(typed, "browser")
	if why != "" {
		t.Fatalf("a good code was refused: %s", why)
	}
	if got.Token != link.Token || got.ID != link.ID {
		t.Fatalf("redeemed a different credential: %+v", got)
	}
	// Nothing was created: the roster still holds one device.
	if n := len(m.Devices()); n != 1 {
		t.Errorf("roster grew to %d; a share code must create nothing", n)
	}
	if got.Scope != "guest" || len(got.ViewPanes) != 1 {
		t.Errorf("the link's own scope did not come with it: %+v", got)
	}
}

func TestShareCodeWorksOnceAndSaysSo(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)
	code, _ := m.MintShareCode(link.ID)

	if _, why := m.RedeemWhy(code, "browser"); why != "" {
		t.Fatalf("first redeem refused: %s", why)
	}
	if _, why := m.RedeemWhy(code, "browser"); why != RedeemUsed {
		t.Errorf("second redeem said %q, want %q", why, RedeemUsed)
	}
}

func TestShareCodeRunsOut(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)
	code, _ := m.MintShareCode(link.ID)

	now = now.Add(shareCodeTTL + time.Second)
	if _, why := m.RedeemWhy(code, "browser"); why != RedeemExpired {
		t.Errorf("an old code said %q, want %q", why, RedeemExpired)
	}
}

// The code is a door to a room, not a key of its own: once the link is gone, so is it.
func TestShareCodeDiesWithItsLink(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, link := guestManager(t, &now)
	code, _ := m.MintShareCode(link.ID)

	if !m.Revoke(link.ID) {
		t.Fatal("the link could not be revoked")
	}
	if _, why := m.RedeemWhy(code, "browser"); why == "" {
		t.Error("a code for a revoked link still handed over a token")
	}
}

func TestShareCodeRefusedForAnythingButALiveLink(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, _ := guestManager(t, &now)

	if _, ok := m.MintShareCode("nosuch"); ok {
		t.Error("minted a code for a link that does not exist")
	}
	// An owner device is not a share link: its token is full control, and a code is not
	// how that gets handed to anyone.
	owner, _ := m.Redeem(m.Mint(), "iPhone")
	if _, ok := m.MintShareCode(owner.ID); ok {
		t.Error("minted a share code for an owner device")
	}
	// An expired link hands over nothing either.
	past := m.MintGuest("yesterday", []string{"%12"}, nil, now.Add(-time.Hour).Unix())
	if _, ok := m.MintShareCode(past.ID); ok {
		t.Error("minted a code for an expired link")
	}
}

// An owner pairing code still goes through the same endpoint, and still creates a device.
func TestPairingCodesAreUntouched(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m, _ := guestManager(t, &now)
	d, why := m.RedeemWhy(m.Mint(), "browser")
	if why != "" || d.Scope == "guest" {
		t.Fatalf("an owner code no longer pairs an owner device: %q %+v", why, d)
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
	// The four characters Crockford leaves out are the ones that get misheard.
	if strings.ContainsAny(code, "ILOU") {
		t.Errorf("code holds an ambiguous character: %q", code)
	}
}

// The whole path a person walks: the owner mints a code for a link, the guest types it
// on the bare page, and what comes back is that link's credential — not a new device,
// and not the owner's.
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

	// A guest cannot mint a door for anyone: minting is the owner's act.
	if rr := post("/api/share/code", `{"id":"`+link.ID+`"}`, link.Token); rr.Code != http.StatusForbidden {
		t.Fatalf("guest minting a code = %d, want 403", rr.Code)
	}

	rr := post("/api/share/code", `{"id":"`+link.ID+`"}`, testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("owner minting a code = %d (%s)", rr.Code, rr.Body.String())
	}
	var minted struct {
		Code         string `json:"code"`
		ExpiresInSec int    `json:"expiresInSec"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &minted); err != nil || minted.Code == "" {
		t.Fatalf("no code in %s", rr.Body.String())
	}
	if minted.ExpiresInSec <= 0 || minted.ExpiresInSec > 3600 {
		t.Errorf("ttl %ds is not the ten minutes a person is told about", minted.ExpiresInSec)
	}

	// The guest types it, lower case and without the dash, on the unauthenticated door.
	typed := strings.ToLower(strings.ReplaceAll(minted.Code, "-", ""))
	rr = post("/api/enroll", `{"enrollCode":"`+typed+`","name":"browser"}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("redeeming the code = %d (%s)", rr.Code, rr.Body.String())
	}
	var got struct{ Token, DeviceID string }
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Token != link.Token {
		t.Fatalf("redeemed a different token than the link's")
	}
	if n := len(em.Devices()); n != 1 {
		t.Errorf("roster grew to %d: a share code must create nothing", n)
	}

	// And it is spent.
	if rr := post("/api/enroll", `{"enrollCode":"`+typed+`","name":"browser"}`, ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("a second redeem = %d, want 401", rr.Code)
	}
}

// Six characters is only a safe trade while nobody can sit there trying codes. The
// limiter is what buys that, so it is tested as part of the code's length, not beside it.
func TestGuessingIsStopped(t *testing.T) {
	em := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: em})
	link := em.MintGuest("probe", nil, nil, 0)
	code, _ := em.MintShareCode(link.ID)
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
		last = try("ZZZZZZ", "203.0.113.9")
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("a run of wrong codes ended in %d, want 429", last)
	}
	// Someone else is not locked out by that, and the right code still works for them.
	if got := try(code, "198.51.100.4"); got != http.StatusOK {
		t.Errorf("a good code from another address = %d, want 200", got)
	}
}

// A person who mistypes once is not closer to being locked out for having then got it
// right: only failures are counted.
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

// The window rolls: a minute later the door opens again on its own.
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
