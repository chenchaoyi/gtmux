package app

import "testing"

// The spawned-window standard handle: "<loc> (%pane) · <title>" — the live tmux number
// (loc) plus the concise purpose, degrading gracefully when parts are missing.
func TestSpawnHandle(t *testing.T) {
	cases := []struct {
		loc, pane, title, want string
	}{
		{"api:2.0", "%18", "fix-auth-mw", "api:2.0 (%18) · fix-auth-mw"},
		{"api:2.0", "%18", "", "api:2.0 (%18)"},         // no title yet
		{"", "%18", "fix-auth-mw", "%18 · fix-auth-mw"}, // loc unknown → bare pane
		{"", "%18", "", "%18"},                          // both unknown → just the pane
	}
	for _, c := range cases {
		if got := spawnHandle(c.loc, c.pane, c.title); got != c.want {
			t.Errorf("spawnHandle(%q,%q,%q) = %q; want %q", c.loc, c.pane, c.title, got, c.want)
		}
	}
}
