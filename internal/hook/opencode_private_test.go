package hook

import "testing"

func TestOpencodePrivateProcessOwnership(t *testing.T) {
	good := map[int]opencodeProcess{10: {20, "opencode", []string{"opencode", "serve", "--stdio", "--port", "0"}}, 20: {30, "opencode", []string{"opencode", "--standalone", "--session", "ses_one"}}}
	for _, tc := range []struct {
		name   string
		change func(map[int]opencodeProcess)
		want   bool
	}{
		{"leased private server", func(map[int]opencodeProcess) {}, true},
		{"shared service", func(m map[int]opencodeProcess) {
			p := m[10]
			p.args = []string{"opencode", "serve", "--service"}
			m[10] = p
		}, false},
		{"inherited launcher", func(m map[int]opencodeProcess) { p := m[20]; p.args = []string{"opencode"}; m[20] = p }, false},
		{"foreground manual server", func(m map[int]opencodeProcess) { delete(m, 20) }, false},
		{"external server client", func(m map[int]opencodeProcess) { p := m[20]; p.args = append(p.args, "--server"); m[20] = p }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := map[int]opencodeProcess{}
			for k, v := range good {
				m[k] = v
			}
			tc.change(m)
			if got := opencodePrivateChain(10, func(pid int) opencodeProcess { return m[pid] }); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}
