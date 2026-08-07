package app

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixtures below are REAL lines, copied byte-for-byte out of
// ~/.local/share/tmux/resurrect/ on the machine that reported the bug (the save
// taken at 07:22, 2.5 hours before the reboot that produced six phantom agent
// sessions). They are the ground truth this fix is measured against, so they are
// pinned here rather than paraphrased.
const (
	// A live Claude Code pane. Note pane_current_command is the agent's VERSION,
	// not its name — the identity has to come from the full command.
	realLiveAgentLine = "pane\tAurora\t1\t0\t:##\t0\t✳ 优化配置加载性能\t:/Users/x/proj/aurora-mobile\t1\t2.1.220\t:claude --resume d644ae48-4379-41f7-abf9-fe4bb23627df"
	// A plain shell pane (someone's extra terminal inside a project).
	realShellLine = "pane\tMP\t0\t0\t:##-\t1\tdev-mbp.local\t:/Users/x/proj/companion\t0\tbash\t:"
	// THE reported pane: routine binary upgrades in a bare shell — and a SHIFTED
	// line (empty pane title), so its command sits one field to the left and the
	// trailing "full command" field is resurrect's own garbage.
	realShiftedShellLine = "pane\t日常更新\t0\t1\t:*\t0\t:/Users/x\t1\tbash\t77304\t:"
)

func TestParseSavedPaneLine_normalLayout(t *testing.T) {
	sp, ok := parseSavedPaneLine(realLiveAgentLine)
	if !ok {
		t.Fatal("a normal pane line must parse")
	}
	if sp.Loc != "Aurora:1.0" {
		t.Errorf("Loc = %q, want Aurora:1.0", sp.Loc)
	}
	if sp.Dir != "/Users/x/proj/aurora-mobile" {
		t.Errorf("Dir = %q", sp.Dir)
	}
	if sp.Cmd != "2.1.220" {
		t.Errorf("Cmd = %q, want the version string (this is the trap)", sp.Cmd)
	}
	if sp.Full != "claude --resume d644ae48-4379-41f7-abf9-fe4bb23627df" {
		t.Errorf("Full = %q", sp.Full)
	}
	if sp.Shifted {
		t.Error("this line is not shifted")
	}
}

// The empty-title field shift: bash's `read` collapses the run of tabs, so every
// field after the title moves left by one. Reading the fixed column here would call
// the pane's PID its command — "77304" is not a shell, so the pane would sail
// through the liveness gate and get a conversation injected. That is precisely how
// the reported pane kept coming back with `claude --resume` typed into it.
func TestParseSavedPaneLine_shiftedLayout(t *testing.T) {
	sp, ok := parseSavedPaneLine(realShiftedShellLine)
	if !ok {
		t.Fatal("a shifted pane line must still parse")
	}
	if !sp.Shifted {
		t.Fatal("the shift must be detected")
	}
	if sp.Loc != "日常更新:0.0" {
		t.Errorf("Loc = %q", sp.Loc)
	}
	if sp.Dir != "/Users/x" {
		t.Errorf("Dir = %q, want /Users/x", sp.Dir)
	}
	if sp.Cmd != "bash" {
		t.Errorf("Cmd = %q, want bash (NOT the pid that sits in the fixed column)", sp.Cmd)
	}
	if sp.Full != "" {
		t.Errorf("Full = %q — a shifted line's full command describes another process and must be dropped", sp.Full)
	}
	if got := sp.evidence(); got != evidenceShell {
		t.Fatalf("evidence = %v, want shell — this is THE regression guard", got)
	}
}

func TestParseSavedPaneLine_rejects(t *testing.T) {
	for _, line := range []string{
		"window\tMP\t0\t:MP\tlayout",             // not a pane line
		"pane\t\t0\t1\t:*\t0\t:/d\t1\tbash\t:",   // no session name
		"pane\tMP\t0\t1\t:*\t0",                  // truncated
		"pane\tMP\t0\t1\tx\t0\tt\td\t1\tbash\t:", // no ':' marks the dir → unknown layout
	} {
		if _, ok := parseSavedPaneLine(line); ok {
			t.Errorf("must not parse: %q", line)
		}
	}
}

// A pane title that itself starts with ':' (bash title prologues write exactly
// that) must NOT be mistaken for the shifted layout — index 7 decides.
func TestParseSavedPaneLine_colonTitleIsNotAShift(t *testing.T) {
	line := "pane\tAurora\t0\t0\t:-\t0\t:/Users/x/some/title\t:/Users/x/work\t1\tbash\t:"
	sp, ok := parseSavedPaneLine(line)
	if !ok {
		t.Fatal("must parse")
	}
	if sp.Shifted {
		t.Fatal("a ':'-prefixed TITLE is not a field shift")
	}
	if sp.Dir != "/Users/x/work" || sp.Cmd != "bash" {
		t.Fatalf("dir/cmd read from the wrong columns: %+v", sp)
	}
}

func TestParseSavedPaneLine_escapedSpacesInDir(t *testing.T) {
	line := "pane\tS\t0\t0\t:-\t0\ttitle\t:/Users/x/My\\ Docs\t1\tbash\t:"
	sp, _ := parseSavedPaneLine(line)
	if sp.Dir != "/Users/x/My Docs" {
		t.Errorf("Dir = %q, want the un-escaped path", sp.Dir)
	}
}

func TestSavedPaneEvidence(t *testing.T) {
	cases := []struct {
		name string
		sp   savedPane
		want paneEvidence
	}{
		{"live claude", savedPane{Cmd: "2.1.220", Full: "claude --resume abc"}, evidenceAgent},
		{"claude with no id", savedPane{Cmd: "2.1.220", Full: "claude"}, evidenceAgent},
		{"agent via interpreter", savedPane{Cmd: "node", Full: "node /opt/bin/codex resume abc"}, evidenceAgent},
		{"idle shell", savedPane{Cmd: "bash", Full: ""}, evidenceShell},
		{"login shell", savedPane{Cmd: "-zsh", Full: ""}, evidenceShell},
		{"shell with a full command", savedPane{Cmd: "bash", Full: "bash"}, evidenceShell},
		{"a non-agent program", savedPane{Cmd: "vim", Full: "vim notes.md"}, evidenceOther},
		{"agent by command name alone", savedPane{Cmd: "codex", Full: ""}, evidenceAgent},
		// A shifted line for a LIVE claude: no full command, and the version string
		// names nothing we can look up. Unclear must ALLOW — the mirror-image bug
		// (losing a conversation that really was running) is the worse failure.
		{"unnameable but running", savedPane{Cmd: "2.1.220", Full: "", Shifted: true}, evidenceUnclear},
		{"nothing recorded", savedPane{}, evidenceUnclear},
	}
	for _, c := range cases {
		if got := c.sp.evidence(); got != c.want {
			t.Errorf("%s: evidence = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEvidenceAllowsResume(t *testing.T) {
	allow := map[paneEvidence]bool{
		evidenceAgent:   true,
		evidenceUnclear: true,
		evidenceShell:   false,
		evidenceOther:   false,
		evidenceMissing: false,
	}
	for ev, want := range allow {
		if got := ev.allowsResume(); got != want {
			t.Errorf("%v.allowsResume() = %v, want %v", ev, got, want)
		}
	}
}

func TestSavedSessionID(t *testing.T) {
	sp, _ := parseSavedPaneLine(realLiveAgentLine)
	agent, id := sp.savedSessionID()
	if agent != "claude" || id != "d644ae48-4379-41f7-abf9-fe4bb23627df" {
		t.Fatalf("savedSessionID = %q/%q", agent, id)
	}
	shell, _ := parseSavedPaneLine(realShellLine)
	if a, i := shell.savedSessionID(); a != "" || i != "" {
		t.Fatalf("a shell pane carries no conversation, got %q/%q", a, i)
	}
}

func TestLoadSavedLayout(t *testing.T) {
	dir := t.TempDir()
	save := filepath.Join(dir, "save.txt")
	body := realLiveAgentLine + "\n" + realShellLine + "\n" + realShiftedShellLine + "\n" +
		"window\tMP\t0\t:MP\tlayout\n"
	if err := os.WriteFile(save, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l := loadSavedLayout(save)
	if len(l.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(l.Panes))
	}
	if l.Ref.IsZero() {
		t.Error("Ref should carry the save's mtime")
	}
	if _, ok := l.ByLoc["日常更新:0.0"]; !ok {
		t.Fatalf("locator index missing the reported pane: %v", l.ByLoc)
	}
	if got := l.ByLoc["Aurora:1.0"].evidence(); got != evidenceAgent {
		t.Errorf("Aurora:1.0 evidence = %v, want agent", got)
	}
	if got := l.ByLoc["MP:0.1"].evidence(); got != evidenceShell {
		t.Errorf("MP:0.1 evidence = %v, want shell", got)
	}

	// A missing save yields an EMPTY layout — the callers' signal that there is no
	// evidence to gate on, so they must fall back to their pre-gate behavior rather
	// than refusing every resume.
	if l := loadSavedLayout(filepath.Join(dir, "nope.txt")); len(l.Panes) != 0 {
		t.Error("a missing save must yield no panes")
	}
	if l := loadSavedLayout(""); len(l.Panes) != 0 || len(l.ByLoc) != 0 {
		t.Error("an empty path must yield an empty layout")
	}
}

// The incident, replayed: the pre-reboot save had 21 panes of which exactly 10 ran
// an agent; restore brought back 16. Every pane the commander named as a phantom
// must be classified as a shell.
func TestIncidentSaveClassification(t *testing.T) {
	phantoms := map[string]string{
		"日常更新:0.0":      realShiftedShellLine,
		"MP:0.1":        realShellLine,
		"gtmux dev:0.1": "pane\tgtmux dev\t0\t1\t:*\t1\t:/Users/x/proj/gtmux\t1\tbash\t75856\t:",
		"HQ:1.0":        "pane\tHQ\t1\t0\t:##\t0\t:/Users/x/proj/gtmux-wt/feat-hq-distill-trigger\t1\tbash\t77885\t:",
		"Aurora:2.0":    "pane\tAurora\t2\t1\t:*\t0\t:/private/tmp\t1\tbash\t70016\t:",
	}
	for loc, line := range phantoms {
		sp, ok := parseSavedPaneLine(line)
		if !ok {
			t.Fatalf("%s: line must parse", loc)
		}
		if sp.Loc != loc {
			t.Errorf("locator = %q, want %q", sp.Loc, loc)
		}
		if ev := sp.evidence(); ev.allowsResume() {
			t.Errorf("%s: evidence %v allows a resume — this pane was a bare shell and must be refused", loc, ev)
		}
	}
}
