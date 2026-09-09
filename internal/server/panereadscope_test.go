package server

import (
	"net/http"
	"strings"
	"testing"
)

// readServer wires every pane-scoped read with content that names the pane it came
// from, so a leak is visible in the body rather than inferred from a status code.
func readServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	enroll := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:     enroll,
		AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
		Diff:       func(id string) (string, error) { return "WORKING-TREE-OF-" + id, nil },
		Transcript: func(id string) ([]byte, TranscriptMeta, error) {
			return []byte(`[{"text":"CONVERSATION-OF-` + id + `"}]`), TranscriptMeta{}, nil
		},
		HasPendingAsk: func(string) bool { return true },
		PaneText: func(id string) (string, bool) {
			// The pane id goes in the OPTION LABEL, not just the question: the options
			// route answers with parsed labels only, so a marker in the prose above them
			// would make a leak invisible to this test.
			return "  Deploy to production?\n\n  ❯ 1. Yes, PROJECT-" + id + "\n    2. No, exit\n", true
		},
	})
	g := enroll.MintGuest("alice", []string{"%1"}, nil, 0)
	return s.Handler(), g.Token
}

// paneReads are the routes that answer with the CONTENT of a named pane. A link that
// names the panes it may see has to bound all of them, not the one somebody remembered.
var paneReads = []struct{ name, path, secret string }{
	{"diff", "/api/diff?id=", "WORKING-TREE-OF-"},
	{"transcript", "/api/transcript?id=", "CONVERSATION-OF-"},
	{"options", "/api/options?id=", "PROJECT-"},
	{"pane screen", "/api/pane?id=", "PROJECT-"},
}

// A guest link names the panes it may see. These routes took the pane id from the query
// and answered, so a link scoped to one pane read the working-tree diff, the agent's
// whole conversation, and the pending question of every other pane on the machine.
//
// The assertion is on the BODY, not the status: what must not happen is the content
// arriving, and a route that starts refusing with 200 and an empty payload would be a
// different bug, not a pass.
func TestAGuestCannotReadAPaneOutsideItsLink(t *testing.T) {
	h, guestTok := readServer(t)
	for _, c := range paneReads {
		t.Run(c.name, func(t *testing.T) {
			rr := do(t, h, http.MethodGet, c.path+"%252", guestTok)
			if body := rr.Body.String(); strings.Contains(body, c.secret+"%2") {
				t.Errorf("a link scoped to %%1 read %s of %%2: %s", c.name, body)
			}
			if rr.Code != http.StatusForbidden {
				t.Errorf("%s of an unshared pane = %d, want 403", c.name, rr.Code)
			}
		})
	}
}

// The guest's OWN pane still answers, or the link stopped being a link.
func TestAGuestStillReadsItsOwnPane(t *testing.T) {
	h, guestTok := readServer(t)
	for _, c := range paneReads {
		t.Run(c.name, func(t *testing.T) {
			rr := do(t, h, http.MethodGet, c.path+"%251", guestTok)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s of the SHARED pane = %d, want 200", c.name, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), c.secret+"%1") {
				t.Errorf("%s of the shared pane returned nothing useful: %s", c.name, rr.Body.String())
			}
		})
	}
}

// The owner is not scoped by a share link.
func TestTheOwnerReadsEveryPane(t *testing.T) {
	h, _ := readServer(t)
	for _, c := range paneReads {
		rr := do(t, h, http.MethodGet, c.path+"%252", testToken)
		if rr.Code != http.StatusOK {
			t.Errorf("master %s = %d, want 200", c.name, rr.Code)
		}
	}
}
