package hook

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// TestSummarizeBackground checks the count + label extraction from Claude's
// Stop-payload background_tasks array.
func TestSummarizeBackground(t *testing.T) {
	cases := []struct {
		name      string
		tasks     []backgroundTask
		wantCount int
		wantLabel string
	}{
		{"empty", nil, 0, ""},
		{
			"one running shell → command is the label",
			[]backgroundTask{{Type: "shell", Status: "running", Command: "npm run dev", Description: "dev server"}},
			1, "npm run dev",
		},
		{
			"shell command preferred over an earlier non-shell",
			[]backgroundTask{
				{Type: "monitor", Status: "running", Description: "watching files"},
				{Type: "shell", Status: "running", Command: "go test ./..."},
			},
			2, "go test ./...",
		},
		{
			"no shell → first description",
			[]backgroundTask{{Type: "subagent", Status: "pending", Description: "researching"}},
			1, "researching",
		},
		{
			"terminal statuses are not counted",
			[]backgroundTask{
				{Type: "shell", Status: "completed", Command: "done cmd"},
				{Type: "shell", Status: "running", Command: "live cmd"},
			},
			1, "live cmd",
		},
		{
			"all terminal → zero",
			[]backgroundTask{{Type: "shell", Status: "failed", Command: "x"}},
			0, "",
		},
		{
			"unknown status counts as in-flight",
			[]backgroundTask{{Type: "workflow", Status: "weird-state", Description: "wf"}},
			1, "wf",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotCount, gotLabel := summarizeBackground(c.tasks)
			if gotCount != c.wantCount || gotLabel != c.wantLabel {
				t.Errorf("summarizeBackground = (%d, %q), want (%d, %q)",
					gotCount, gotLabel, c.wantCount, c.wantLabel)
			}
		})
	}
}

// TestSummarizeBackgroundLabelCap checks the one-line label is capped.
func TestSummarizeBackgroundLabelCap(t *testing.T) {
	long := strings.Repeat("x", bgLabelMax+40)
	_, label := summarizeBackground([]backgroundTask{{Type: "shell", Status: "running", Command: long}})
	if len([]rune(label)) > bgLabelMax+1 { // +1 for the ellipsis
		t.Errorf("label not capped: len=%d", len([]rune(label)))
	}
	if !strings.HasSuffix(label, "…") {
		t.Errorf("capped label should end with ellipsis, got %q", label)
	}
}

// TestRunStopWritesBackgroundMarker: a Claude Stop whose background_tasks has a
// live shell records the bg marker (count + command); an empty array clears it.
func TestRunStopWritesBackgroundMarker(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%3")

	Run(strings.NewReader(`{"hook_event_name":"Stop","session_id":"s1",
		"background_tasks":[{"type":"shell","status":"running","command":"npm run dev"}]}`), nil)

	n, label := state.ReadBackground("%3")
	if n != 1 || label != "npm run dev" {
		t.Fatalf("bg marker = (%d, %q), want (1, %q)", n, label, "npm run dev")
	}
	// The row is still idle (Stop stamps finished).
	if !state.Exists(state.FinishedPath("%3")) {
		t.Error("Stop should stamp the finished marker")
	}

	// A later Stop with no background work clears the bg marker.
	Run(strings.NewReader(`{"hook_event_name":"Stop","session_id":"s1","background_tasks":[]}`), nil)
	if n, _ := state.ReadBackground("%3"); n != 0 {
		t.Fatalf("empty background_tasks should clear the marker, got count=%d", n)
	}
}

// TestRunStopNoBackgroundField: a plain Stop (no background_tasks key, e.g. an
// older Claude or a non-Claude agent) writes no bg marker.
func TestRunStopNoBackgroundField(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%4")
	Run(strings.NewReader(`{"hook_event_name":"Stop","session_id":"s2"}`), nil)
	if n, _ := state.ReadBackground("%4"); n != 0 {
		t.Fatalf("no background_tasks → no marker, got count=%d", n)
	}
}

// TestRunNextTurnClearsBackgroundMarker: once a bg-marked pane starts a new turn
// (UserPromptSubmit → not idle), the modifier is dropped.
func TestRunNextTurnClearsBackgroundMarker(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("TMUX_PANE", "%7")

	if err := state.WriteBackground("%7", 2, "go build"); err != nil {
		t.Fatal(err)
	}
	Run(strings.NewReader(`{"hook_event_name":"UserPromptSubmit","session_id":"s3"}`), nil)
	if n, _ := state.ReadBackground("%7"); n != 0 {
		t.Fatalf("a new turn should clear the bg marker, got count=%d", n)
	}
}
