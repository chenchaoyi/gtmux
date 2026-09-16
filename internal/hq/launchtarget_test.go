package hq

import "testing"

// The three flags are one instruction; the checks here are the ones a wrong answer
// would turn into typing an agent command somewhere the user did not mean.
func TestResolveLaunchTarget(t *testing.T) {
	cases := []struct {
		name    string
		t       launchTarget
		own     string
		kind    string
		pane    string
		wantErr bool
	}{
		{"nothing asked", launchTarget{}, "%1", "", "", false},
		{"a named pane", launchTarget{pane: "%31"}, "", "pane", "%31", false},
		{"a named pane must be a pane id", launchTarget{pane: "hq:0.0"}, "", "", "", true},
		{"here is the pane the command runs in", launchTarget{here: true}, "%31", "pane", "%31", false},
		{"here outside tmux is an error, not a guess", launchTarget{here: true}, "", "", "", true},
		{"new pane splits the window the command runs in", launchTarget{newPane: true}, "%31", "split", "%31", false},
		{"new pane outside tmux is an error", launchTarget{newPane: true}, "", "", "", true},
		{"two flags are a contradiction", launchTarget{here: true, newPane: true}, "%31", "", "", true},
	}
	for _, c := range cases {
		kind, pane, err := resolveLaunchTarget(c.t, c.own)
		if (err != nil) != c.wantErr || kind != c.kind || pane != c.pane {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q, err=%v)", c.name, kind, pane, err, c.kind, c.pane, c.wantErr)
		}
	}
}

func TestShq(t *testing.T) {
	if got := shq("/Users/o'brien/.config/gtmux/hq"); got != `'/Users/o'\''brien/.config/gtmux/hq'` {
		t.Fatalf("shq = %s", got)
	}
}
