package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestClassifyAgent(t *testing.T) {
	p := builtinProfiles
	cases := []struct {
		name       string
		title, cmd string
		wantAgent  bool
		wantStatus string
		wantName   string
		wantTask   string
	}{
		{
			name:  "braille spinner is working regardless of command",
			title: "⠋ refactoring auth", cmd: "node",
			wantAgent: true, wantStatus: "working", wantName: "agent", wantTask: "refactoring auth",
		},
		{
			name:  "claude version command, running at prompt",
			title: "Claude Code", cmd: "2.1.177",
			wantAgent: true, wantStatus: "running", wantName: "Claude Code", wantTask: "",
		},
		{
			name:  "claude idle marker with a task",
			title: "✳ Build the thing", cmd: "claude",
			wantAgent: true, wantStatus: "idle", wantName: "Claude Code", wantTask: "Build the thing",
		},
		{
			name:  "stale agent title over a plain shell does NOT count",
			title: "✳ Claude Code", cmd: "bash",
			wantAgent: false,
		},
		{
			name:  "plain shell, no agent",
			title: "", cmd: "zsh",
			wantAgent: false,
		},
		{
			name:  "named agent by command, running",
			title: "codex session", cmd: "codex",
			wantAgent: true, wantStatus: "running", wantName: "Codex", wantTask: "codex session",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isAgent, agent, status, task := classifyAgent(c.title, c.cmd, p)
			if isAgent != c.wantAgent {
				t.Fatalf("isAgent = %v, want %v", isAgent, c.wantAgent)
			}
			if !c.wantAgent {
				return
			}
			if status != c.wantStatus {
				t.Errorf("status = %q, want %q", status, c.wantStatus)
			}
			if agent != c.wantName {
				t.Errorf("agent = %q, want %q", agent, c.wantName)
			}
			if task != c.wantTask {
				t.Errorf("task = %q, want %q", task, c.wantTask)
			}
		})
	}
}

func TestCmdIsLiveAgent(t *testing.T) {
	p := builtinProfiles
	cases := []struct {
		cmd      string
		wantName string
		wantLive bool
	}{
		{"claude", "Claude Code", true},
		{"codex", "Codex", true},
		{"cursor-agent", "Cursor", true},
		{"2.1.177", "Claude Code", true}, // version string ⇒ Claude
		{"bash", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		name, live := cmdIsLiveAgent(c.cmd, p)
		if live != c.wantLive || name != c.wantName {
			t.Errorf("cmdIsLiveAgent(%q) = (%q, %v), want (%q, %v)", c.cmd, name, live, c.wantName, c.wantLive)
		}
	}
}

func TestStatusRank(t *testing.T) {
	if !(statusRank("waiting") < statusRank("working")) {
		t.Error("waiting must sort before working")
	}
	if !(statusRank("working") < statusRank("idle")) {
		t.Error("working must sort before idle")
	}
	if statusRank("idle") != statusRank("running") {
		t.Error("idle and running share the lowest priority")
	}
}

// sortPanes: status groups first, then the finished (idle) group most-recently-
// finished first (by `since` desc), other groups by location.
func TestSortPanes(t *testing.T) {
	panes := []agentPane{
		{loc: "z-idle", status: "idle", since: 100},
		{loc: "a-idle", status: "idle", since: 300}, // finished most recently
		{loc: "m-idle", status: "idle", since: 200},
		{loc: "b-working", status: "working", since: 50},
		{loc: "a-waiting", status: "waiting", since: 10},
	}
	sortPanes(panes)
	got := make([]string, len(panes))
	for i, p := range panes {
		got[i] = p.loc
	}
	// waiting, then working, then the three idle by since desc (a=300, m=200, z=100).
	want := []string{"a-waiting", "b-working", "a-idle", "m-idle", "z-idle"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// waitMarkStale: a waiting mark is obsolete when the pane's activity postdates it by
// more than the grace (agent resumed, or a reused pane id inherited an orphan mark).
func TestWaitMarkStale(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.WaitingDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	pane := "%13"
	path := state.WaitingPath(pane)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mark := time.Now().Add(-9 * 24 * time.Hour) // 9 days ago
	if err := os.Chtimes(path, mark, mark); err != nil {
		t.Fatal(err)
	}
	markUnix := mark.Unix()

	// activity long after the mark → stale (the "waiting 9d" orphan case).
	if !waitMarkStale(pane, time.Now().Unix()) {
		t.Error("activity 9d after the mark should be stale")
	}
	// activity at/near the mark → a real, live wait, not stale.
	if waitMarkStale(pane, markUnix) {
		t.Error("activity == mark should not be stale")
	}
	if waitMarkStale(pane, markUnix+waitStaleGrace-1) {
		t.Error("activity within the grace should not be stale")
	}
	// unknown activity, or no mark → never stale.
	if waitMarkStale(pane, 0) {
		t.Error("unknown activity should not be stale")
	}
	if waitMarkStale("%999", time.Now().Unix()) {
		t.Error("a pane with no mark should not be stale")
	}
}

func TestResolveWaiting(t *testing.T) {
	// live is a thunk that records whether it was consulted (the frame/CPU sample
	// must be taken ONLY when a working title sits over a fresh mark).
	live := func(working bool, consulted *bool) func() bool {
		return func() bool { *consulted = true; return working }
	}
	cases := []struct {
		name       string
		status     string
		hasMark    bool
		stale      bool
		liveWork   bool
		wantStatus string
		wantClear  bool
		wantConsul bool // whether liveWorking should have been called
	}{
		// The bug: a real permission prompt whose pane TITLE still shows a (frozen)
		// spinner. Fresh mark + NOT live-working → waiting, mark kept (card shows).
		{"frozen-spinner prompt", "working", true, false, false, "waiting", false, true},
		// Answered → the approved tool is running (pane demonstrably live-working).
		// Show working; the mark is left for PostToolUse→Resumed / staleness to clear.
		{"answered, tool running", "working", true, false, true, "working", false, true},
		// A Claude idle/ready pane with a fresh mark → waiting without sampling frames.
		{"idle with fresh mark", "idle", true, false, false, "waiting", false, false},
		{"running with fresh mark", "running", true, false, false, "waiting", false, false},
		// Stale orphan mark → keep the raw status, clear the mark. Never sample frames.
		{"stale orphan, working", "working", true, true, false, "working", true, false},
		{"stale orphan, idle", "idle", true, true, false, "idle", true, false},
		// No mark → status untouched, nothing cleared, frames never sampled.
		{"no mark, working", "working", false, false, false, "working", false, false},
		{"no mark, idle", "idle", false, false, false, "idle", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			consulted := false
			got, clear := resolveWaiting(c.status, c.hasMark, c.stale, live(c.liveWork, &consulted))
			if got != c.wantStatus {
				t.Errorf("status = %q, want %q", got, c.wantStatus)
			}
			if clear != c.wantClear {
				t.Errorf("clearMark = %v, want %v", clear, c.wantClear)
			}
			if consulted != c.wantConsul {
				t.Errorf("liveWorking consulted = %v, want %v", consulted, c.wantConsul)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Errorf("firstNonEmpty = %q, want %q", got, "third")
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty(all empty) = %q, want empty", got)
	}
}

func TestAgentsSummary(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")

	if got := agentsSummary(nil); got != "0 agents" {
		t.Errorf("empty summary = %q, want %q", got, "0 agents")
	}
	panes := []agentPane{
		{status: "waiting"}, {status: "working"}, {status: "idle"}, {status: "running"},
	}
	want := "4 agents · 1 waiting · 1 working · 2 idle" // running counts toward idle bucket
	if got := agentsSummary(panes); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// loadProfiles must prepend user-defined profiles (from
// $HOME/.config/gtmux/agents.json) ahead of the built-ins.
func TestLoadProfilesUserOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `[{"name":"MyAgent","commands":["myagent"],"idleGlyph":"★"}]`
	if err := os.WriteFile(filepath.Join(dir, "agents.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	got := loadProfiles()
	if len(got) != len(builtinProfiles)+1 {
		t.Fatalf("got %d profiles, want %d", len(got), len(builtinProfiles)+1)
	}
	if got[0].Name != "MyAgent" {
		t.Errorf("first profile = %q, want user entry %q", got[0].Name, "MyAgent")
	}
	// And the user command is recognized as a live agent.
	if name, live := cmdIsLiveAgent("myagent", got); !live || name != "MyAgent" {
		t.Errorf("cmdIsLiveAgent(myagent) = (%q, %v), want (MyAgent, true)", name, live)
	}
}

func TestParseCPUTime(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"0:00.00", 0},
		{"12.50", 12.5},
		{"1:30.00", 90},
		{"168:49.35", 168*60 + 49.35},
		{"1:02:03.00", 3723},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseCPUTime(c.in); got != c.want {
			t.Errorf("parseCPUTime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
