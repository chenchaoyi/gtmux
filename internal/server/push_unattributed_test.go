package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A stored push token that says neither which device registered it nor that the serve's
// own token did is UNATTRIBUTED. Before #470 no token carried a device id, and before
// registration was owner-only a share link could register (v0.28.0 accepted a guest's
// register and stored it with no id; %12's history check, 2026-10-06). Nothing in such a
// record tells an owner's phone from a guest's, so it is kept and paused, and the owner's
// app makes it attributed again by registering, which it does on launch, on returning to
// the foreground and on a settings change.

// legacyStore is a push-tokens.json written before deviceId/origin existed.
func legacyStore(t *testing.T) []DeviceToken {
	t.Helper()
	var toks []DeviceToken
	if err := json.Unmarshal([]byte(`[{"token":"old-tok","platform":"ios","env":"sandbox"}]`), &toks); err != nil {
		t.Fatal(err)
	}
	return toks
}

func unattributedFixture(t *testing.T) pushFixture {
	t.Helper()
	f := newPushFixture(t)
	for _, d := range legacyStore(t) {
		f.pm.Register(d)
	}
	return f
}

func sendAll(f pushFixture) {
	f.pm.dispatch(Alert{Kind: "waiting", Agent: "x", Pane: "%1"})
	f.pm.pushBadge(1)
	f.pm.Test()
}

func countTo(r *fakeRelay, tok string) int {
	n := 0
	for _, s := range sentTo(r) {
		if s == tok {
			n++
		}
	}
	return n
}

func TestPush_AnUnattributedTokenIsKeptButNotSentTo(t *testing.T) {
	f := unattributedFixture(t)
	sendAll(f)
	if n := countTo(f.relay, "old-tok"); n != 0 {
		t.Fatalf("an unattributed token was sent %d pushes", n)
	}
	if len(f.pm.Tokens()) != 1 {
		t.Fatal("an unattributed token must be kept, not deleted")
	}
}

func TestPush_TheOwnerRegisteringAgainResumesIt(t *testing.T) {
	for _, who := range []string{"paired device", "serve token"} {
		t.Run(who, func(t *testing.T) {
			f := unattributedFixture(t)
			bearer := f.owner
			if who == "serve token" {
				bearer = testToken
			}
			if rr := post(t, f.h, "/api/push/register", bearer, `{"token":"old-tok","platform":"ios","env":"sandbox"}`); rr.Code != http.StatusOK {
				t.Fatalf("register = %d %s", rr.Code, rr.Body.String())
			}
			sendAll(f)
			if n := countTo(f.relay, "old-tok"); n != 3 {
				t.Fatalf("after the owner registered again, sent %d pushes, want 3 (alert, badge, test)", n)
			}
		})
	}
}

func TestPush_AShareLinkCannotClaimAnUnattributedToken(t *testing.T) {
	f := unattributedFixture(t)
	if rr := post(t, f.h, "/api/push/register", f.guest, `{"token":"old-tok","platform":"ios","origin":"master"}`); rr.Code != http.StatusForbidden {
		t.Fatalf("guest register = %d, want 403", rr.Code)
	}
	sendAll(f)
	if n := countTo(f.relay, "old-tok"); n != 0 {
		t.Fatalf("a share link's attempt made the token sendable: %d pushes", n)
	}
}

// Neither field is read from the body: a paired device saying it is the serve's own
// token is still recorded as itself.
func TestPush_TheBodyCannotClaimAnOrigin(t *testing.T) {
	f := newPushFixture(t)
	if rr := post(t, f.h, "/api/push/register", f.owner, `{"token":"dev-tok","origin":"master","deviceId":"someone-else"}`); rr.Code != http.StatusOK {
		t.Fatalf("register = %d", rr.Code)
	}
	toks := f.pm.Tokens()
	if len(toks) != 1 || toks[0].DeviceID != f.ownerID || toks[0].Origin != "" {
		t.Fatalf("stored %+v, want the caller's own device id and no origin", toks)
	}
}

// "orphans" clears exactly the unattributed tokens, never the serve's own.
func TestPush_ForgettingOrphansKeepsTheServesOwnTokens(t *testing.T) {
	f := unattributedFixture(t)
	if rr := post(t, f.h, "/api/push/register", testToken, `{"token":"own-tok","platform":"ios"}`); rr.Code != http.StatusOK {
		t.Fatalf("register = %d", rr.Code)
	}
	if n := f.pm.Forget("", true, false); n != 1 {
		t.Fatalf("forget orphans removed %d, want 1 (the unattributed one)", n)
	}
	if toks := f.pm.Tokens(); len(toks) != 1 || toks[0].Token != "own-tok" {
		t.Fatalf("left %+v, want only the serve's own token", toks)
	}
}

// The attribution a registration stamps is written to the store and survives a restart:
// what is saved is read back with its origin, and stays sendable.
func TestPush_AnAttributionSurvivesARestart(t *testing.T) {
	var saved []byte
	pm := NewPushManager(&fakeRelay{}, legacyStore(t), func(d []DeviceToken) {
		saved, _ = json.Marshal(d)
	}, "Mac", nil)
	enroll := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: enroll, Share: NewShareManager(ShareState{}, nil), Push: pm})
	if rr := post(t, s.Handler(), "/api/push/register", testToken, `{"token":"old-tok","platform":"ios"}`); rr.Code != http.StatusOK {
		t.Fatalf("register = %d", rr.Code)
	}
	var reloaded []DeviceToken
	if err := json.Unmarshal(saved, &reloaded); err != nil || len(reloaded) != 1 || reloaded[0].Origin != originMaster {
		t.Fatalf("saved %s, want the token with origin %q", saved, originMaster)
	}
	relay := &fakeRelay{}
	after := NewPushManager(relay, reloaded, nil, "Mac", nil)
	after.pushBadge(1)
	if countTo(relay, "old-tok") != 1 {
		t.Fatal("a reloaded attributed token must be sent to")
	}
}

// /api/push/tokens says which tokens nothing is sent to: unattributed ones and those bound
// to a share link or a device no longer on the roster, the same check every send makes.
func TestPush_TheTokenListSaysWhatIsPaused(t *testing.T) {
	f := unattributedFixture(t)
	f.pm.Register(DeviceToken{Token: "g-tok", DeviceID: f.guestID})
	f.pm.Register(DeviceToken{Token: "gone-tok", DeviceID: "no-such-device"})
	f.pm.Register(DeviceToken{Token: "dev-tok", DeviceID: f.ownerID})
	f.pm.Register(DeviceToken{Token: "own-tok", Origin: originMaster})
	rr := do(t, f.h, http.MethodGet, "/api/push/tokens", testToken)
	var out struct {
		Tokens []struct {
			DeviceID string `json:"deviceId"`
			Origin   string `json:"origin"`
			Paused   bool   `json:"paused"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("tokens: %v %s", err, rr.Body.String())
	}
	paused := map[string]bool{}
	for _, r := range out.Tokens {
		key := r.DeviceID
		if key == "" {
			key = "origin:" + r.Origin
		}
		paused[key] = r.Paused
	}
	want := map[string]bool{"origin:": true, f.guestID: true, "no-such-device": true, f.ownerID: false, "origin:master": false}
	for k, v := range want {
		if paused[k] != v {
			t.Errorf("%s paused=%v, want %v (all: %v)", k, paused[k], v, paused)
		}
	}
}
