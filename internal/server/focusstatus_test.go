package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/state"
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
		{"headless host", fmt.Errorf("wrapped: %w", panefocus.ErrUnsupported), http.StatusNotImplemented},
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

func TestUnsupportedFocusIsARefusalRatherThanAnOperationalError(t *testing.T) {
	for _, tc := range []struct {
		name, level, outcome string
		err                  error
	}{
		{"unsupported", "warn", diag.Refused, panefocus.ErrUnsupported},
		{"terminal failure", "error", diag.Failed, errors.New("terminal failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := New(Config{Token: testToken}, Deps{Focus: func(string) error { return tc.err }})
			post(t, s.Handler(), "/api/focus?id=%251", testToken, `{}`)
			files, err := filepath.Glob(filepath.Join(state.LogsDir(), "*.jsonl"))
			if err != nil || len(files) == 0 {
				t.Fatalf("missing action receipt: %v", err)
			}
			found := 0
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					var entry diag.Entry
					if err := json.Unmarshal([]byte(line), &entry); err != nil {
						t.Fatal(err)
					}
					if entry.Event == "act.focus" {
						found++
						if entry.Level != tc.level || entry.Outcome != tc.outcome {
							t.Fatalf("incorrect diagnostic classification: %+v", entry)
						}
					}
				}
			}
			if found != 1 {
				t.Fatalf("focus wrote %d action receipts, want 1", found)
			}
		})
	}
}
