package app

import (
	"encoding/json"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

func TestWorkspaceOverviewIsExplicitlyRequestedAndDoesNotEnlargeSidebarMatters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	s := hqworkspace.New(t.TempDir())
	_, err := s.Register("goal", "Existing goal", "Original report", "needs-input", hqworkspace.Origin{Kind: "task", Ref: "original-task"}, 100)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := workspaceSnapshot(s, false, 200)
	if err != nil || ordinary.Overview != nil || ordinary.Matters[0].Assessment != "" {
		t.Fatal(ordinary, err)
	}
	brief, err := workspaceSnapshot(s, true, 200)
	if err != nil || brief.Overview == nil || brief.Matters[0].Assessment != "" {
		t.Fatal(brief, err)
	}
	p := brief.Overview.NeedsInput[0].Preview
	if p == nil || p.Body != "Original report" || p.MessageID == "" || p.At != 100 {
		t.Fatal("lost explicit source", p)
	}
	wire, err := json.Marshal(brief)
	if err != nil || !json.Valid(wire) {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"show", "global", "--overview"}, {"list", "unexpected", "--overview"}} {
		if cmdHQWorkspace(args) != 2 {
			t.Fatal("overview flag escaped list-only validation", args)
		}
	}
}
