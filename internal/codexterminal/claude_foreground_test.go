package codexterminal

import "testing"

func TestClaudeVersionAliasRequiresLinuxForegroundGroupLeader(t *testing.T) {
	for _, tc := range []struct {
		name, platform, reported, stdin, want string
		pid, parent, group                    int
	}{
		{"Linux launcher alias", "linux", "claude", "/dev/pts/7", "", 200, 100, 200},
		{"Darwin exact version", "darwin", "2.1.123", "/dev/pts/7", "", 200, 100, 200},
		{"Darwin alias rejected", "darwin", "claude", "/dev/pts/7", "client-unverified", 200, 100, 200},
		{"non-leader in foreground group", "linux", "claude", "/dev/pts/7", "client-unverified", 201, 100, 200},
		{"background group leader", "linux", "claude", "/dev/pts/7", "client-unverified", 201, 100, 201},
		{"unrelated process", "linux", "claude", "/dev/pts/7", "client-unverified", 200, 1, 200},
		{"different stdin", "linux", "claude", "/dev/pts/8", "terminal-mismatch", 200, 100, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pane := Pane{ID: "%7", PID: 100, Command: tc.reported, TTY: "/dev/pts/7"}
			tree := map[int]process{
				100:    {pid: 100, parent: 1, group: 100, foreground: 200, tty: "pts/7", start: "root-birth", command: "bash"},
				tc.pid: {pid: tc.pid, parent: tc.parent, group: tc.group, foreground: 200, tty: "pts/7", start: "client-birth", command: "2.1.123"},
			}
			i := inspector{
				snapshot: func() map[int]process { return tree },
				stdin:    func(int) string { return tc.stdin },
				pane:     func(Pane) bool { return true },
			}
			o := i.observeWith([]Pane{pane}, func(p Pane, procs map[int]process) (process, []int, string) {
				return inspectClaude(p, procs, tc.platform)
			})[pane.ID]
			if o.Code != tc.want {
				t.Fatalf("observation=%+v, want diagnostic %q", o, tc.want)
			}
			if tc.want == "" && o.Client != (Client{PID: tc.pid, Start: "client-birth", RootStart: "root-birth", TTY: "pts/7"}) {
				t.Fatalf("verified alias lost its terminal or process proof: %+v", o)
			}
			if tc.want != "" && o.PID != 0 {
				t.Fatalf("refused alias returned a live client: %+v", o)
			}
		})
	}
}
