package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestWorkspaceTriageRoleAfterPaneReuse(t *testing.T) {
	r := newWorkspaceCLIReplay(t)
	s := workspaceStore()
	if err := s.ActivateObservations(events.CurrentSeq(), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	paneAt := func(name, dir string) string {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for {
			pane, err := tmux.Run("new-session", "-d", "-s", name, "-c", dir, "-P", "-F", "#{pane_id}", "sleep 60")
			if err == nil {
				return pane
			}
			if time.Now().After(deadline) {
				t.Fatal(pane, err)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	hook := func(pane, id, cwd string) {
		t.Helper()
		cmd := exec.Command(r.exe, "-test.run=^TestWorkspaceTriageCLI$")
		cmd.Env = append(os.Environ(), "TMUX_PANE="+pane, "GTMUX_TRIAGE_FIXTURE=hook")
		cmd.Dir = cwd
		body, err := json.Marshal(map[string]string{"hook_event_name": "Stop", "session_id": id, "cwd": cwd})
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdin = strings.NewReader(string(body))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook: %v: %s", err, out)
		}
	}
	const workerID = "old-worker"
	const hqID = "new-supervisor"
	old := paneAt("worker", r.home)
	hook(old, workerID, r.home)
	if !tmux.OK("kill-server") {
		t.Fatal("failed to restart the task's private tmux")
	}
	hq := paneAt("hq", state.HQHome())
	if old != hq {
		t.Fatal("fixture requires reused numeric pane", old, hq)
	}
	hqpane.Stamp(hq)
	hook(hq, hqID, state.HQHome())
	rows, gap := events.ReadSince(0)
	if gap {
		t.Fatal("fixture lost the source interval")
	}
	var workerStop, hqStop events.Record
	for _, row := range rows {
		if row.Event != "Stop" {
			continue
		}
		switch row.AgentSession {
		case workerID:
			workerStop = row
		case hqID:
			hqStop = row
		}
	}
	if workerStop.Seq == 0 || hqStop.Seq <= workerStop.Seq || workerStop.Pane != hqStop.Pane {
		t.Fatal(workerStop, hqStop)
	}
	if workerStop.TerminalRole != events.TerminalRoleWorker || hqStop.TerminalRole != events.TerminalRoleHQ {
		t.Fatal("source role was not captured with the exact event", workerStop.TerminalRole, hqStop.TerminalRole)
	}
	if err := ingestWorkspaceObservations(s, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	q := d.Reconciliations["event-"+strconv.FormatInt(workerStop.Seq, 10)]
	if q.Observation.Conversation.SessionID != workerID || q.Observation.Kind != "triage-unassigned:Stop" {
		t.Fatalf("reused HQ pane discarded the historical worker Stop (cursor %d): %+v", d.ObservationCursors["events"], q)
	}
	t.Logf("worker %s at events:%d and HQ %s at events:%d reused %s; workspace cursor=%d", workerID, workerStop.Seq, hqID, hqStop.Seq, hq, d.ObservationCursors["events"])
	if _, ok := d.Reconciliations["event-"+strconv.FormatInt(hqStop.Seq, 10)]; ok || d.ObservationCursors["events"] < hqStop.Seq {
		t.Fatal("HQ echo counted or cursor did not advance", d.Reconciliations)
	}
}
