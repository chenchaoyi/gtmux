package server

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Push is the owner's: a share link may not register for alerts or Live Activity, and a
// revoked device stops being sent to, its activity token included. Found by the
// 2026-10-06 docs/web audit on an isolated serve with a fake relay: a link that could
// view one pane registered a token and then received the task text of a pane outside
// its view; and a revoked device's activity token kept getting tally updates.

type pushFixture struct {
	h            http.Handler
	pm           *PushManager
	relay        *fakeRelay
	enroll       *EnrollManager
	guest, owner string // bearer tokens
	guestID      string
	ownerID      string
}

func newPushFixture(t *testing.T) pushFixture {
	t.Helper()
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "Mac", nil)
	enroll := NewEnrollManager(nil, nil)
	share := NewShareManager(ShareState{}, nil)
	g := enroll.MintGuest("g", []string{"%0"}, nil, 0)
	dev, ok := enroll.Redeem(enroll.Mint(), "phone")
	if !ok {
		t.Fatal("enroll a device")
	}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: enroll, Share: share, Push: pm})
	return pushFixture{h: s.Handler(), pm: pm, relay: relay, enroll: enroll, guest: g.Token, owner: dev.Token, guestID: g.ID, ownerID: dev.ID}
}

// sentTo lists the tokens the relay was asked to send to, in order.
func sentTo(r *fakeRelay) []string {
	var out []string
	for _, i := range r.intents() {
		out = append(out, i.Token)
	}
	return out
}

func TestPush_AShareLinkCannotRegister(t *testing.T) {
	f := newPushFixture(t)
	for _, path := range []string{"/api/push/register", "/api/push/activity", "/api/push/unregister"} {
		if rr := post(t, f.h, path, f.guest, `{"token":"g-tok","activityToken":"g-act"}`); rr.Code != http.StatusForbidden {
			t.Errorf("guest POST %s = %d %s, want 403", path, rr.Code, rr.Body.String())
		}
	}
	if n := len(f.pm.Tokens()); n != 0 {
		t.Fatalf("a refused guest left %d tokens registered", n)
	}
	if acts, _ := f.pm.ActivityStatus(); len(acts) != 0 {
		t.Fatalf("a refused guest left %d activity tokens", len(acts))
	}
	// The owner's own device still registers both.
	if rr := post(t, f.h, "/api/push/register", f.owner, `{"token":"o-tok","platform":"ios"}`); rr.Code != http.StatusOK {
		t.Fatalf("owner register = %d %s", rr.Code, rr.Body.String())
	}
	if rr := post(t, f.h, "/api/push/activity", f.owner, `{"token":"o-act","env":"sandbox"}`); rr.Code != http.StatusOK {
		t.Fatalf("owner activity = %d %s", rr.Code, rr.Body.String())
	}
}

// A guest token already in the store (registered before registration was owner-only)
// is skipped at send time and kept, not deleted: it stays inspectable and removable with
// `gtmux devices --forget-push`. Unlinked legacy tokens are sent to as before.
func TestPush_ATokenAShareLinkRegisteredEarlierIsNeverSentTo(t *testing.T) {
	f := newPushFixture(t)
	f.pm.Register(DeviceToken{Token: "g-tok", Platform: "ios", DeviceID: f.guestID})
	f.pm.Register(DeviceToken{Token: "o-tok", Platform: "ios", DeviceID: f.ownerID})
	f.pm.Register(DeviceToken{Token: "legacy-tok", Platform: "ios"})
	f.pm.dispatch(Alert{Kind: "waiting", Agent: "Claude Code", Task: "SYNTH_TASK_ON_%1", Pane: "%1"})
	f.pm.pushBadge(1)
	if n := f.pm.Test(); n != 2 {
		t.Errorf("test push tried %d devices, want 2 (owner + unlinked)", n)
	}
	for _, tok := range sentTo(f.relay) {
		if tok == "g-tok" {
			t.Fatalf("a share link's token was sent to: %v", sentTo(f.relay))
		}
	}
	if got := strings.Join(sentTo(f.relay), ","); !strings.Contains(got, "o-tok") || !strings.Contains(got, "legacy-tok") {
		t.Fatalf("the owner's and the unlinked token must still be sent to, got %s", got)
	}
	if n := len(f.pm.Tokens()); n != 3 {
		t.Fatalf("the store holds %d tokens, want 3: a skipped token is kept, not deleted", n)
	}
}

// Revoking a device drops its activity token with its alert token; the next tally
// update goes to the devices that remain.
func TestPush_RevokingADeviceDropsItsActivityToken(t *testing.T) {
	f := newPushFixture(t)
	if rr := post(t, f.h, "/api/push/activity", f.owner, `{"token":"o-act","env":"sandbox"}`); rr.Code != http.StatusOK {
		t.Fatalf("owner activity = %d", rr.Code)
	}
	f.pm.RegisterActivity("master-act", "sandbox") // the serve's own token: unlinked
	waitSent := func(want int) {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if len(f.relay.intents()) >= want {
				return
			}
		}
		t.Fatalf("relay got %d intents, want %d", len(f.relay.intents()), want)
	}
	f.pm.PushLiveActivity(Tally{Waiting: 1, WaitingTitle: "SYNTH"})
	waitSent(2)

	if rr := post(t, f.h, "/api/devices/revoke", testToken, `{"id":"`+f.ownerID+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("revoke = %d %s", rr.Code, rr.Body.String())
	}
	if acts, _ := f.pm.ActivityStatus(); acts["o-act"] != (ActivityInfo{}) {
		t.Fatalf("the revoked device's activity token is still held: %+v", acts)
	}
	before := len(f.relay.intents())
	f.pm.PushLiveActivity(Tally{Waiting: 2, WaitingTitle: "SYNTH-2"})
	waitSent(before + 1)
	time.Sleep(50 * time.Millisecond) // a second send would have arrived by now
	for _, i := range f.relay.intents()[before:] {
		if i.Token == "o-act" {
			t.Fatal("a revoked device's activity token was sent a tally update")
		}
	}
}

// revokingRelay revokes a device during the first send it is handed, the timing %12's
// re-verification used: every later recipient must be checked again before its own send.
type revokingRelay struct {
	fakeRelay
	revoke func()
	once   bool
}

func (r *revokingRelay) Send(i PushIntent) error {
	if !r.once && r.revoke != nil {
		r.once = true
		r.revoke()
	}
	return r.fakeRelay.Send(i)
}

func TestPush_EachRecipientIsCheckedRightBeforeItsSend(t *testing.T) {
	for _, path := range []string{"alert", "badge", "test"} {
		t.Run(path, func(t *testing.T) {
			enroll := NewEnrollManager(nil, nil)
			a, _ := enroll.Redeem(enroll.Mint(), "a")
			b, _ := enroll.Redeem(enroll.Mint(), "b")
			relay := &revokingRelay{}
			pm := NewPushManager(relay, nil, nil, "Mac", nil)
			pm.SetEligible(enroll.IsOwnerDevice)
			pm.Register(DeviceToken{Token: "tok-a", Platform: "ios", DeviceID: a.ID})
			pm.Register(DeviceToken{Token: "tok-b", Platform: "ios", DeviceID: b.ID})
			// Whichever is sent first revokes the other.
			relay.revoke = func() {
				first := relay.intents()
				if len(first) == 0 {
					enroll.Revoke(a.ID)
					enroll.Revoke(b.ID)
				}
			}
			var attempted int
			switch path {
			case "alert":
				pm.dispatch(Alert{Kind: "waiting", Agent: "x", Pane: "%1"})
			case "badge":
				pm.pushBadge(1)
			case "test":
				attempted = pm.Test()
			}
			if got := sentTo(&relay.fakeRelay); len(got) != 1 {
				t.Fatalf("%s: sent to %v, want only the first recipient: the second was revoked before its send", path, got)
			}
			if path == "test" && attempted != 1 {
				t.Errorf("Test() reported %d attempts, want 1", attempted)
			}
		})
	}
}
