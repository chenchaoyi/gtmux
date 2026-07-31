package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// A resurrect save line for one pane: session, window index, pane index. The
// title/cwd/command fields are filled with plausible placeholders — the plan only
// reads fields 1 (session), 2 (window), 5 (pane).
func savePane(session, win, pane string) string {
	return "pane\t" + session + "\t" + win + "\t1\t:\t" + pane + "\t✳ title\t:/some/dir\t1\tclaude\t123\t:claude"
}

func TestBuildRestorePlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Save: session "MP" has windows 0 (panes 0,1) and 1 (pane 0) = 2 windows / 3
	// panes; session "solo" has one shell pane.
	save := filepath.Join(t.TempDir(), "resurrect.txt")
	body := savePane("MP", "0", "0") + "\n" +
		savePane("MP", "0", "1") + "\n" +
		savePane("MP", "1", "0") + "\n" +
		savePane("solo", "0", "0") + "\n" +
		"window\tMP\t0\t:MP\t...\n" // a non-pane line is ignored
	if err := os.WriteFile(save, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Resume records: two live agents at MP:0.0 and MP:1.0, plus a STALE MP:0.2
	// whose pane no longer exists in the save — it must be dropped.
	_ = resume.Save("MP:0.0", resume.Record{Agent: "claude", SessionID: "s-00", Cwd: "/p/mp", UpdatedAt: 300})
	_ = resume.Save("MP:1.0", resume.Record{Agent: "codex", SessionID: "s-10", Cwd: "/p/mp", UpdatedAt: 200})
	_ = resume.Save("MP:0.2", resume.Record{Agent: "claude", SessionID: "s-stale", Cwd: "/p/mp", UpdatedAt: 100})

	plan := buildRestorePlanFrom(save)

	if len(plan.Sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(plan.Sessions))
	}
	mp := plan.Sessions[0] // first appearance order
	if mp.Name != "MP" || mp.Windows != 2 || mp.Panes != 3 {
		t.Fatalf("MP counts wrong: %+v", mp)
	}
	// The stale MP:0.2 record must NOT appear — only the two panes still in the save.
	if len(mp.Agents) != 2 {
		t.Fatalf("MP should list 2 agents (stale MP:0.2 dropped), got %d: %+v", len(mp.Agents), mp.Agents)
	}
	ids := map[string]bool{}
	for _, a := range mp.Agents {
		ids[a.SessionID] = true
	}
	if ids["s-stale"] {
		t.Fatal("the stale locator's conversation must not be in the plan")
	}
	if !ids["s-00"] || !ids["s-10"] {
		t.Fatalf("both live agents should be present, got %v", ids)
	}

	if solo := plan.Sessions[1]; solo.Name != "solo" || len(solo.Agents) != 0 {
		t.Fatalf("solo should be an agent-less session: %+v", solo)
	}

	if plan.agentCount() != 2 {
		t.Fatalf("agentCount = %d, want 2", plan.agentCount())
	}
	// s-00 is claude with NO transcript on disk → dead (×); s-10 is codex → always
	// resumable (we can't inspect its store). So the headline should count 1 resumable
	// and note 1 with-no-transcript, matching the eventual "resumed 1".
	if got := plan.resumableCount(); got != 1 {
		t.Fatalf("resumableCount = %d, want 1 (only the codex agent is resumable)", got)
	}
	if got := plan.deadCount(); got != 1 {
		t.Fatalf("deadCount = %d, want 1 (the claude agent has no transcript)", got)
	}
}

func TestBuildRestorePlan_NoSave(t *testing.T) {
	if p := buildRestorePlanFrom(""); len(p.Sessions) != 0 || p.SavePath != "" {
		t.Fatalf("empty save → empty plan, got %+v", p)
	}
}

func TestLocSession(t *testing.T) {
	cases := map[string]string{
		"MP:0.1":        "MP",
		"a:b:2.3":       "a:b", // colon in session name → split on LAST colon
		"gtmux dev:1.0": "gtmux dev",
	}
	for in, want := range cases {
		if got := locSession(in); got != want {
			t.Errorf("locSession(%q) = %q, want %q", in, got, want)
		}
	}
}
