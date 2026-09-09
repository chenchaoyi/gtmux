package server

import (
	"net/http"
	"testing"
)

// scopedServer wires the deps the pane routes need and returns a handler plus a guest
// token whose link may see %1 and nothing else.
func scopedServer(t *testing.T, focused *string) (http.Handler, string) {
	t.Helper()
	enroll := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:     enroll,
		AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
		Focus: func(id string) error {
			if focused != nil {
				*focused = id
			}
			return nil
		},
	})
	g := enroll.MintGuest("alice", []string{"%1"}, nil, 0)
	return s.Handler(), g.Token
}

// A guest link names the panes it may see. Focus moves the OWNER's terminal to a pane,
// so a guest naming a pane outside its link pulls the owner's screen somewhere the link
// was never given. It discloses nothing to the guest; it takes the owner's screen away
// from what they were doing, and it can be repeated.
func TestFocusRefusesAPaneOutsideTheLink(t *testing.T) {
	var focused string
	h, guestTok := scopedServer(t, &focused)

	rr := post(t, h, "/api/focus?id=%252", guestTok, `{}`)
	if rr.Code != http.StatusForbidden {
		t.Errorf("guest focus of an unshared pane = %d, want 403", rr.Code)
	}
	// The status is the mechanism; not moving the owner's terminal is the property.
	if focused != "" {
		t.Errorf("the owner's terminal was moved to %q by a guest who may not see it", focused)
	}
}

// The shared pane still works, or the guest's own link stopped doing its job.
func TestFocusAllowsTheSharedPane(t *testing.T) {
	var focused string
	h, guestTok := scopedServer(t, &focused)

	if rr := post(t, h, "/api/focus?id=%251", guestTok, `{}`); rr.Code != http.StatusOK {
		t.Fatalf("guest focus of its OWN shared pane = %d, want 200", rr.Code)
	}
	if focused != "%1" {
		t.Errorf("focused %q, want %%1", focused)
	}
}

// The owner is not scoped by a share link.
func TestFocusUnrestrictedForMaster(t *testing.T) {
	var focused string
	h, _ := scopedServer(t, &focused)

	if rr := post(t, h, "/api/focus?id=%252", testToken, `{}`); rr.Code != http.StatusOK {
		t.Fatalf("master focus = %d, want 200", rr.Code)
	}
	if focused != "%2" {
		t.Errorf("focused %q, want %%2", focused)
	}
}
