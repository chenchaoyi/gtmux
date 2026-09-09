package server

import (
	"net/http"
	"testing"
)

// countingRelay records every push it is asked to send.
type countingRelay struct{ sent []PushIntent }

func (c *countingRelay) Send(p PushIntent) error { c.sent = append(c.sent, p); return nil }

// POST /api/push/test rings EVERY registered device — that is what makes it a useful
// "did push survive the last change?" button, and what makes it the owner's button.
// A guest calling it reaches the owner's phone, wherever the owner is, as often as it
// asks. Nothing arrives at the guest, so this is the nuisance tier rather than a leak,
// but it is still the owner's surface.
func TestAGuestCannotRingTheOwnersPhone(t *testing.T) {
	relay := &countingRelay{}
	enroll := NewEnrollManager(nil, nil)
	push := NewPushManager(relay, nil, nil, "Mac", nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Enroll: enroll, Push: push})
	h := s.Handler()

	dev, _ := enroll.Redeem(enroll.Mint(), "owners-phone")
	if rr := post(t, h, "/api/push/register", dev.Token, `{"token":"OWNER-APNS","platform":"ios"}`); rr.Code != http.StatusOK {
		t.Fatalf("owner register = %d", rr.Code)
	}
	guest := enroll.MintGuest("alice", []string{"%1"}, nil, 0)

	if rr := post(t, h, "/api/push/test", guest.Token, `{}`); rr.Code != http.StatusForbidden {
		t.Errorf("guest POST /api/push/test = %d, want 403", rr.Code)
	}
	// The status is the mechanism; the owner's phone staying quiet is the property.
	if len(relay.sent) != 0 {
		t.Errorf("a guest rang the owner's devices %d time(s): %+v", len(relay.sent), relay.sent)
	}

	// The owner's own button still works, or push became untestable.
	if rr := post(t, h, "/api/push/test", testToken, `{}`); rr.Code != http.StatusOK {
		t.Fatalf("master POST /api/push/test = %d, want 200", rr.Code)
	}
	if len(relay.sent) != 1 {
		t.Errorf("master test sent %d pushes, want 1", len(relay.sent))
	}
}
