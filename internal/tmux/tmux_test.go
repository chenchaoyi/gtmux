package tmux

import (
	"os"
	"os/exec"
	"testing"
)

// withEmptyBin temporarily blanks the resolved tmux binary so the documented
// "no tmux" early-return paths can be exercised without execing a real tmux.
// (The package guarantees a zero value / error when Bin == "".)
func withEmptyBin(t *testing.T) {
	t.Helper()
	saved := Bin
	Bin = ""
	t.Cleanup(func() { Bin = saved })
}

// TestInTmux is the pure env predicate: TMUX set (non-empty) ⇒ true.
func TestInTmux(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		want bool
	}{
		{"unset → false", false, "", false},
		{"empty → false", true, "", false},
		{"set to socket path → true", true, "/tmp/tmux-501/default,1234,0", true},
		{"set to arbitrary non-empty → true", true, "x", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv("TMUX", c.val)
			} else {
				// Setenv then unset to guarantee a clean slate regardless of host env.
				t.Setenv("TMUX", "placeholder")
				if err := unsetenv("TMUX"); err != nil {
					t.Fatalf("unset TMUX: %v", err)
				}
			}
			if got := InTmux(); got != c.want {
				t.Errorf("InTmux() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestRunNoBin: with no tmux binary, Run returns ("", exec.ErrNotFound) and never
// shells out.
func TestRunNoBin(t *testing.T) {
	withEmptyBin(t)
	out, err := Run("has-session")
	if out != "" {
		t.Errorf("Run out = %q, want empty", out)
	}
	if err != exec.ErrNotFound {
		t.Errorf("Run err = %v, want exec.ErrNotFound", err)
	}
}

// TestOKNoBin: OK reports false when there is no tmux binary.
func TestOKNoBin(t *testing.T) {
	withEmptyBin(t)
	if OK("has-session") {
		t.Error("OK with empty Bin must be false")
	}
}

// TestServerUpNoBin: ServerUp is false when there is no tmux binary.
func TestServerUpNoBin(t *testing.T) {
	withEmptyBin(t)
	if ServerUp() {
		t.Error("ServerUp with empty Bin must be false")
	}
}

// TestLinesNoBin: Lines returns nil (not a zero-length non-nil slice) on error.
func TestLinesNoBin(t *testing.T) {
	withEmptyBin(t)
	if got := Lines("list-panes"); got != nil {
		t.Errorf("Lines = %#v, want nil", got)
	}
}

// TestCapturePaneNoBin: both capture variants degrade to "" without tmux.
func TestCapturePaneNoBin(t *testing.T) {
	withEmptyBin(t)
	if got := CapturePane("%1"); got != "" {
		t.Errorf("CapturePane = %q, want empty", got)
	}
	if got := CapturePaneColor("%1"); got != "" {
		t.Errorf("CapturePaneColor = %q, want empty", got)
	}
}

// TestSendTextNoBin: SendText surfaces the no-tmux error when there is text to
// type; with empty text and no Enter it is a no-op that still succeeds (nothing
// is sent, so nothing can fail).
func TestSendTextNoBin(t *testing.T) {
	withEmptyBin(t)
	cases := []struct {
		name    string
		text    string
		enter   bool
		wantErr error
	}{
		{"text typed → ErrNotFound", "hello", false, exec.ErrNotFound},
		{"text + enter → ErrNotFound (text leg fails first)", "hi", true, exec.ErrNotFound},
		{"empty text, enter requested → ErrNotFound (Enter leg)", "", true, exec.ErrNotFound},
		{"empty text, no enter → nil no-op", "", false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := SendText("%1", c.text, c.enter)
			if err != c.wantErr {
				t.Errorf("SendText(%q, enter=%v) err = %v, want %v", c.text, c.enter, err, c.wantErr)
			}
		})
	}
}

// TestSendKeyNoBin: SendKey surfaces the no-tmux error.
func TestSendKeyNoBin(t *testing.T) {
	withEmptyBin(t)
	if err := SendKey("%1", "Enter"); err != exec.ErrNotFound {
		t.Errorf("SendKey err = %v, want exec.ErrNotFound", err)
	}
}

// TestDisplayNoBin: Display returns "" without tmux, for both targeted and
// untargeted forms (exercising the optional -t branch in arg building).
func TestDisplayNoBin(t *testing.T) {
	withEmptyBin(t)
	if got := Display("", "#{session_name}"); got != "" {
		t.Errorf("Display(no target) = %q, want empty", got)
	}
	if got := Display("%1", "#{pane_id}"); got != "" {
		t.Errorf("Display(target) = %q, want empty", got)
	}
}

// unsetenv fully clears an env var. We call t.Setenv first (in the caller) so the
// testing framework restores the original value on cleanup; os.Unsetenv just
// blanks it for the duration of this test.
func unsetenv(key string) error {
	return os.Unsetenv(key)
}
