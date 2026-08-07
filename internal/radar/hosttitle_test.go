package radar

import "testing"

// tmux's default pane_title is the machine's hostname. An agent that never sets a
// title (codex) must not surface the hostname as its session name — the title is
// blanked so Task stays empty and clients fall back to the tmux session name.
func TestStripDefaultTitle(t *testing.T) {
	const host = "MBP-FYVW37QLPV-1819.local"
	cases := []struct {
		name, title, want string
	}{
		{"exact hostname", "MBP-FYVW37QLPV-1819.local", ""},
		{"hostname sans .local", "MBP-FYVW37QLPV-1819", ""},
		{"case-insensitive", "mbp-fyvw37qlpv-1819.LOCAL", ""},
		{"padded", "  MBP-FYVW37QLPV-1819.local  ", ""},
		{"real agent title survives", "✳ Claude Code", "✳ Claude Code"},
		{"ordinary title survives", "vim main.go", "vim main.go"},
		{"prefix is not a match", "MBP-FYVW37QLPV-1819.local extras", "MBP-FYVW37QLPV-1819.local extras"},
		{"empty stays empty", "", ""},
	}
	for _, c := range cases {
		if got := stripDefaultTitle(c.title, host); got != c.want {
			t.Errorf("%s: stripDefaultTitle(%q, %q) = %q, want %q", c.name, c.title, host, got, c.want)
		}
	}
	// An empty hostname must never blank anything.
	if got := stripDefaultTitle("MBP-FYVW37QLPV-1819.local", ""); got != "MBP-FYVW37QLPV-1819.local" {
		t.Errorf("empty host: title was blanked to %q", got)
	}
	// A host WITHOUT .local still matches a .local-suffixed title (and vice versa —
	// covered above): tmux and the OS can disagree on the suffix.
	if got := stripDefaultTitle("dev-mbp.local", "dev-mbp"); got != "" {
		t.Errorf("suffix mismatch direction 2: got %q, want blank", got)
	}
}
