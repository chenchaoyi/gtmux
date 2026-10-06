package panefocus

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// This package had no tests at all — the only package in the repo at 0% — while owning
// the jump that the menu bar, the phone, the web mirror and `gtmux focus` all perform.
// What is testable without a tmux server is the part that matters most: the input guard
// on a REMOTE-reachable entry point, and the promise that every path degrades quietly
// when there is no tmux rather than panicking.

func TestFocusPaneByIDRejectsAnythingThatIsNotAPaneID(t *testing.T) {
	// The id comes from POST /api/focus, so it is remote input. It is validated BEFORE
	// tmux is consulted, which is why these answers do not depend on a running server.
	bad := []string{"", "12", "%", "%%1", "pane1", "%12x", "%12 %13", "%12; tmux kill-server", "%-1", " %12"}
	for _, id := range bad {
		err := FocusPaneByID(id)
		if err == nil {
			t.Errorf("FocusPaneByID(%q) = nil, want an error", id)
			continue
		}
		if !strings.Contains(err.Error(), "not a pane id") {
			t.Errorf("FocusPaneByID(%q) failed with %q — the caller is told about the wrong problem", id, err)
		}
	}
}

func TestFocusPaneByIDAcceptsARealPaneIDAndThenAsksTmux(t *testing.T) {
	// A well-formed id passes validation and the answer comes from tmux. With no tmux on
	// this machine the honest answer is that the pane is not there — never "not a pane id".
	old := tmux.Bin
	tmux.Bin = ""
	defer func() { tmux.Bin = old }()
	for _, id := range []string{"%0", "%12", "%99999"} {
		err := FocusPaneByID(id)
		if err == nil {
			t.Fatalf("FocusPaneByID(%q) = nil with no tmux", id)
		}
		if strings.Contains(err.Error(), "not a pane id") {
			t.Errorf("FocusPaneByID(%q) blamed the id: %q", id, err)
		}
	}
}

func TestNothingPanicsWithoutTmux(t *testing.T) {
	// Every surface calls these; a machine without tmux (or with the server stopped) must
	// get a quiet no, not a crash in the menu bar's poll.
	old := tmux.Bin
	tmux.Bin = ""
	defer func() { tmux.Bin = old }()
	JumpPane("%12") // no panic, no output
	if Attached("some-session") {
		t.Error("Attached said yes with no tmux to ask")
	}
	if Attached("") {
		t.Error("Attached said yes for an empty session name")
	}
}

// fakeTerm answers FocusTab/SpawnTabs as a test says and records nothing else.
type fakeTerm struct {
	focus      string
	focusErr   error
	spawnErr   error
	focusCalls int
}

func (f *fakeTerm) Name() string { return "fake" }
func (f *fakeTerm) FocusTab(string) (string, error) {
	f.focusCalls++
	return f.focus, f.focusErr
}
func (f *fakeTerm) IsViewing(string) bool                    { return false }
func (f *fakeTerm) OpenWindow(string) (string, error)        { return "", nil }
func (f *fakeTerm) SpawnTabs([]string, bool) (string, error) { return "", f.spawnErr }
func (f *fakeTerm) TabOrder() []string                       { return nil }

var _ terminal.Terminal = (*fakeTerm)(nil)

func standIn(t *testing.T, term *fakeTerm, attached bool) {
	t.Helper()
	tf, ia := terminalFor, isAttached
	terminalFor = func(string) terminal.Terminal { return term }
	isAttached = func(string) bool { return attached }
	t.Cleanup(func() { terminalFor, isAttached = tf, ia })
}

// A pane that is not there, or an id that is not a pane, is ErrNoPane: the server
// answers 404 for that and for nothing else.
func TestFocusErrorsSayWhetherThePaneIsThere(t *testing.T) {
	if err := FocusPaneByID("%12x"); !errors.Is(err, ErrNoPane) {
		t.Errorf("a malformed id: %v, want ErrNoPane", err)
	}
	old := tmux.Bin
	tmux.Bin = ""
	defer func() { tmux.Bin = old }()
	if err := FocusPaneByID("%12"); !errors.Is(err, ErrNoPane) {
		t.Errorf("no tmux to find it in: %v, want ErrNoPane", err)
	}
}

// The terminal's answer is reported, not dropped (%12, 2026-10-06): a tab that is not
// found used to come back as success.
func TestBringForwardReportsWhatTheTerminalDid(t *testing.T) {
	boom := errors.New("AppleScript failed")
	for _, tc := range []struct {
		name     string
		term     fakeTerm
		attached bool
		want     error
	}{
		{"focused", fakeTerm{focus: "ok"}, true, nil},
		{"no tab shows it", fakeTerm{focus: "notfound"}, true, ErrNoTab},
		{"the terminal failed", fakeTerm{focusErr: boom}, true, boom},
		{"opened a tab for a detached session", fakeTerm{}, false, nil},
		{"could not open one", fakeTerm{spawnErr: boom}, false, boom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := tc.term
			standIn(t, &term, tc.attached)
			_, err := BringForward("sess")
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("BringForward = %v, want %v", err, tc.want)
			}
		})
	}
}

// The whole jump on a real (private) tmux server: the pane is selected, and the
// terminal's outcome is what JumpPane returns.
func TestJumpPaneReturnsTheTerminalsOutcome(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux")
	}
	dir, err := os.MkdirTemp("/tmp", "gtx") // a unix socket path must stay short
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "sock")
	if err := os.MkdirAll(sock, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("HOME", dir)
	run := func(args ...string) string {
		out, _ := tmux.Run(append([]string{"-f", "/dev/null"}, args...)...)
		return strings.TrimSpace(out)
	}
	run("new-session", "-d", "-s", "probe")
	t.Cleanup(func() { run("kill-server") })
	if got := run("list-sessions", "-F", "#{session_name}"); got != "probe" {
		t.Fatalf("not isolated — server holds %q, refusing to go on", got)
	}
	pane := run("display-message", "-p", "-t", "probe", "#{pane_id}")

	term := &fakeTerm{focus: "notfound"}
	standIn(t, term, true)
	if err := JumpPane(pane); !errors.Is(err, ErrNoTab) {
		t.Fatalf("JumpPane = %v, want ErrNoTab", err)
	}
	term.focus = "ok"
	if err := JumpPane(pane); err != nil {
		t.Fatalf("JumpPane = %v, want nil", err)
	}
	if err := JumpPane("%99999"); !errors.Is(err, ErrNoPane) {
		t.Fatalf("a pane that is not there: %v, want ErrNoPane", err)
	}
}
