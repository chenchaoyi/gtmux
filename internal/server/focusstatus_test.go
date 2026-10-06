package server

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/panefocus"
)

// What /api/focus answers is what the jump did (%12, 2026-10-06): a pane that is not
// there is 404, a jump the Mac's terminal could not make is 502, and only a jump that
// reached the screen is 200. The second used to be 200.
func TestFocusStatusSaysWhatTheJumpDid(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"on screen", nil, http.StatusOK},
		{"no such pane", fmt.Errorf("pane %%9: %w", panefocus.ErrNoPane), http.StatusNotFound},
		{"no tab shows it", fmt.Errorf("sess: %w", panefocus.ErrNoTab), http.StatusBadGateway},
		{"the terminal failed", errors.New("AppleScript failed"), http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
				Enroll:     NewEnrollManager(nil, nil),
				AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
				Focus:      func(string) error { return tc.err },
			})
			if rr := post(t, s.Handler(), "/api/focus?id=%251", testToken, `{}`); rr.Code != tc.want {
				t.Fatalf("status %d, want %d (%s)", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}
