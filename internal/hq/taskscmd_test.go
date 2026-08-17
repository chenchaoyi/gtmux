package hq

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

func TestVerboseTail(t *testing.T) {
	// Without --verbose: no tail regardless of fields.
	if got := verboseTail(taskJSON{Disposition: "relayed"}, false); got != "" {
		t.Errorf("non-verbose tail should be empty, got %q", got)
	}
	// With --verbose: the disposition renders.
	if got := verboseTail(taskJSON{Disposition: "relayed"}, true); !strings.Contains(got, "relayed") {
		t.Errorf("verbose tail %q missing the disposition", got)
	}
	// Verbose but no disposition → empty (a plain dispatch stays clean).
	if got := verboseTail(taskJSON{}, true); got != "" {
		t.Errorf("verbose tail with no disposition should be empty, got %q", got)
	}
}

// A dispatch whose pane the radar surfaced as "waiting" — a stuck-before-running worker
// (startup gate / unsubmitted draft, stuck-dispatch-waiting) — maps to the ledger status
// "waiting", NOT "done". `done` stays reserved for a pane that truly went idle after a
// turn, so HQ is never told a task finished when not one step ran.
func TestTaskStatus_StuckIsWaitingNotDone(t *testing.T) {
	live := map[string]radar.Pane{
		"%1": {PaneID: "%1", Status: "waiting"}, // radar flagged it stuck
		"%2": {PaneID: "%2", Status: "idle"},    // genuinely finished a turn
	}
	landed := dispatch.Task{Delivered: true, State: "landed"}
	a, b := landed, landed
	a.Pane, b.Pane = "%1", "%2"
	if got := taskStatus(a, live); got != "waiting" {
		t.Errorf("stuck pane task status = %q, want waiting (never done)", got)
	}
	if got := taskStatus(b, live); got != "done" {
		t.Errorf("genuinely idle pane = %q, want done", got)
	}
}

// The other half of "never done": a dispatch whose goal never LANDED. A ready-gate
// timeout leaves a live, empty, idle agent pane, so a status derived from the pane alone
// rendered it green `✳ done` with the recorded goal — the INTENT — printed beside it.
// The ledger knew (`delivered:false`) and nothing but the resume lookup read it. Three
// dispatches on 2026-08-09 read as finished work having received not one word.
func TestTaskStatus_UndeliveredIsNeverDone(t *testing.T) {
	live := map[string]radar.Pane{
		"%1": {PaneID: "%1", Status: "idle"},    // the failed-spawn shape: alive, empty, idle
		"%2": {PaneID: "%2", Status: "working"}, // a queued delivery that then ran
	}
	failed := dispatch.Task{Pane: "%1", Delivered: false, State: "failed"}
	if got := taskStatus(failed, live); got != radar.TaskStatusUndelivered {
		t.Errorf("failed dispatch on an idle pane = %q, want %q", got, radar.TaskStatusUndelivered)
	}
	// A LEGACY entry predates the state field — `delivered:false` alone must still count.
	legacy := dispatch.Task{Pane: "%1", Delivered: false}
	if got := taskStatus(legacy, live); got != radar.TaskStatusUndelivered {
		t.Errorf("legacy undelivered entry = %q, want %q", got, radar.TaskStatusUndelivered)
	}
	// Queued is NOT undelivered — the agent accepted it, behind the current turn.
	queued := dispatch.Task{Pane: "%2", Delivered: false, State: "queued"}
	if got := taskStatus(queued, live); got != "working" {
		t.Errorf("queued dispatch = %q, want working (accepted, not undelivered)", got)
	}
	// A pane that is gone stays "gone" — there is nothing left to rescue.
	if got := taskStatus(dispatch.Task{Pane: "%9"}, live); got != "gone" {
		t.Errorf("dead pane = %q, want gone", got)
	}
}

// `undelivered` leads the view: a waiting worker is stuck mid-task and a done one
// produced something, but an undelivered dispatch never started at all.
func TestTaskRank_UndeliveredLeads(t *testing.T) {
	order := []string{radar.TaskStatusUndelivered, "waiting", "done", "working", "gone"}
	for i := 1; i < len(order); i++ {
		if taskRank(order[i-1]) >= taskRank(order[i]) {
			t.Errorf("%q must rank ahead of %q", order[i-1], order[i])
		}
	}
	if g, l := taskGlyph(radar.TaskStatusUndelivered); g == "" || l == "" {
		t.Error("undelivered needs its own glyph + label")
	}
}
