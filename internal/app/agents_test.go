package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
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
