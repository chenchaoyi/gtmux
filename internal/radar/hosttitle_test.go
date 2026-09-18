package radar

import "testing"

// tmux's default pane_title is the machine's hostname. An agent that never sets a
// title (codex) must not surface the hostname as its session name — the title is
// blanked so Task stays empty and clients fall back to the tmux session name.
func TestStripDefaultTitle(t *testing.T) {
	const host = "dev-mbp.local"
	cases := []struct {
		name, title, want string
	}{
		{"exact hostname", "dev-mbp.local", ""},
		{"hostname sans .local", "dev-mbp", ""},
		{"case-insensitive", "dev-mbp.LOCAL", ""},
		{"padded", "  dev-mbp.local  ", ""},
		{"real agent title survives", "✳ Claude Code", "✳ Claude Code"},
		{"ordinary title survives", "vim main.go", "vim main.go"},
		{"prefix is not a match", "dev-mbp.local extras", "dev-mbp.local extras"},
		{"empty stays empty", "", ""},
	}
	for _, c := range cases {
		if got := stripDefaultTitle(c.title, host); got != c.want {
			t.Errorf("%s: stripDefaultTitle(%q, %q) = %q, want %q", c.name, c.title, host, got, c.want)
		}
	}
	// An empty hostname must never blank anything.
	if got := stripDefaultTitle("dev-mbp.local", ""); got != "dev-mbp.local" {
		t.Errorf("empty host: title was blanked to %q", got)
	}
	// A host WITHOUT .local still matches a .local-suffixed title (and vice versa —
	// covered above): tmux and the OS can disagree on the suffix.
	if got := stripDefaultTitle("dev-mbp.local", "dev-mbp"); got != "" {
		t.Errorf("suffix mismatch direction 2: got %q, want blank", got)
	}
}
