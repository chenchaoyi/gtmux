package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const triageFixtureSession = "f11ef000-dad0-4f01-9000-000000000016"

func TestWorkspaceTriageCLI(t *testing.T) {
	switch os.Getenv("GTMUX_TRIAGE_FIXTURE") {
	case "hq":
		runTriageFixtureHQ(t)
		return
	case "serve":
		if code := Run([]string{"serve", "--bind", "127.0.0.1", "--port", os.Getenv("GTMUX_TRIAGE_PORT")}); code != 0 {
			t.Fatal("serve exited", code)
		}
		return
	case "hook":
		if code := Run([]string{"hook", "--agent", "claude"}); code != 0 {
			t.Fatal("hook exited", code)
		}
		return
	}

	r := newWorkspaceCLIReplay(t)
	childCommand := shellQuote(r.exe) + " -test.run=^TestWorkspaceTriageCLI$"
	pane, err := tmux.Run("new-session", "-d", "-x", "180", "-y", "35", "-s", "triage-hq", "-c", state.HQHome(),
		"-P", "-F", "#{pane_id}", "-e", "GTMUX_TRIAGE_FIXTURE=hq", "exec "+childCommand)
	if err != nil {
		t.Fatal(pane, err)
	}
	hqpane.Stamp(pane)
	if err := state.WriteMarker(state.ActivePath(pane), "triage-hq-conversation"); err != nil {
		t.Fatal(err)
	}
	waitWorkspaceFixture(t, pane, func() bool { return strings.Contains(tmux.CapturePane(pane), "❯") })
	worker, err := tmux.Run("new-session", "-d", "-s", "triage-worker", "-c", r.home, "-P", "-F", "#{pane_id}", "sleep 180")
	if err != nil {
		t.Fatal(worker, err)
	}
	if err := os.WriteFile(filepath.Join(state.ConfigDir(), "config.json"), []byte(`{"hqWake":{"mineIntervalHours":0,"selfRotateCtx":0,"selfRotateHours":0,"selfRotateTurns":0}}`), 0600); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	serve := exec.Command(r.exe, "-test.run=^TestWorkspaceTriageCLI$")
	serve.Env = append(os.Environ(), "GTMUX_TRIAGE_FIXTURE=serve", "GTMUX_TRIAGE_PORT="+strconv.Itoa(port))
	output, err := os.Create(filepath.Join(r.home, "serve.log"))
	if err != nil {
		t.Fatal(err)
	}
	serve.Stdout, serve.Stderr, serve.Dir = output, output, r.home
	if err = serve.Start(); err != nil {
		output.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = serve.Process.Kill()
		_ = serve.Wait()
		_ = output.Close()
		if t.Failed() {
			b, _ := os.ReadFile(output.Name())
			t.Log(string(b))
		}
	})
	s := workspaceStore()
	read := func() hqworkspace.Database {
		t.Helper()
		d, err := s.Read()
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	waitWorkspaceFixture(t, pane, func() bool { return read().ObservationCursors["relay-start"] > 0 })
	if !tmux.OK("copy-mode", "-t", pane) {
		t.Fatal("hold the first workspace wake before deliberate loss")
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop"} {
		body, err := json.Marshal(map[string]string{"hook_event_name": event, "session_id": triageFixtureSession, "cwd": r.home, "prompt": "Verify the two test markers."})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(r.exe, "-test.run=^TestWorkspaceTriageCLI$")
		cmd.Env = append(os.Environ(), "TMUX_PANE="+worker, "GTMUX_TRIAGE_FIXTURE=hook")
		cmd.Dir = r.home
		cmd.Stdin = strings.NewReader(string(body))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook %s: %v: %s", event, err, out)
		}
	}
	var stop hqworkspace.Observation
	dueFile := ""
	queue := filepath.Join(state.Dir(), "hq-nudges")
	waitWorkspaceFixture(t, pane, func() bool {
		for _, q := range hqworkspace.ReconciliationQueue(read(), time.Now().Unix()) {
			if q.Observation.Kind == "triage-unassigned:Stop" && q.ReminderCount == 1 {
				stop = q.Observation
			}
		}
		entries, _ := os.ReadDir(queue)
		for _, e := range entries {
			b, _ := os.ReadFile(filepath.Join(queue, e.Name()))
			if strings.Contains(string(b), workspaceContinuationWake) && strings.HasSuffix(e.Name(), ".txt") {
				dueFile = filepath.Join(queue, e.Name())
			}
		}
		return stop.ID != "" && dueFile != ""
	})
	wakeReads := filepath.Join(r.home, "triage-terminal-wakes")
	if _, err := os.Stat(wakeReads); !os.IsNotExist(err) {
		t.Fatal("first workspace wake reached the private terminal", err)
	}
	if err := os.Rename(dueFile, filepath.Join(r.home, "dropped-wake.txt")); err != nil {
		t.Fatal(err)
	}
	if !tmux.OK("send-keys", "-t", pane, "-X", "cancel") {
		t.Fatal("leave copy mode")
	}
	if !tmux.OK("send-keys", "-t", pane, "-l", "unsent fixture draft") {
		t.Fatal("put a draft ahead of retry")
	}
	timeLimit := time.Now().Add(48 * time.Second)
	for read().Reconciliations[stop.ID].ReminderCount < 2 && time.Now().Before(timeLimit) {
		time.Sleep(100 * time.Millisecond)
	}
	if read().Reconciliations[stop.ID].ReminderCount != 2 || !strings.Contains(tmux.CapturePane(pane), "unsent fixture draft") {
		t.Fatal("retry failed or touched the private draft")
	}
	if !tmux.OK("send-keys", "-t", pane, "C-u") {
		t.Fatal("clear the task's private draft")
	}
	waitWorkspaceFixture(t, pane, func() bool {
		b, _ := os.ReadFile(filepath.Join(r.home, "triage-cli-read"))
		return strings.Contains(string(b), stop.ID)
	})
	delta, err := os.ReadFile(filepath.Join(r.home, "triage-cli-event-delta"))
	if err != nil || !strings.Contains(string(delta), triageFixtureSession) || read().Reconciliations[stop.ID].State != "pending" {
		t.Fatal("global pull failed or erased triage", string(delta), err)
	}
	var q hqworkspace.Reconciliation
	r.action(t, pane, workspaceAction{Op: "claim", ObservationID: stop.ID}, true, &q)
	if q.Owner != "::triage-hq-conversation" || read().Reconciliations[stop.ID].State != "leased" || len(read().Matters) != 0 {
		t.Fatal("claim is not the exact verified HQ pickup", q)
	}
	r.action(t, pane, workspaceAction{Op: "assess", ID: "generic", ObservationID: stop.ID, Generation: q.Generation, Decision: "noted", ReviewRef: stop.Ref, Body: "Read"}, false, nil)
	const id = "verified-existing-goal"
	var m hqworkspace.Matter
	r.action(t, pane, workspaceAction{Op: "register", ID: id, Title: "Fixture", State: "open", Body: "Reviewed original scope and the incomplete check.", Origin: hqworkspace.Origin{Kind: "conversation", Ref: stop.Ref}}, true, &m)
	r.action(t, pane, workspaceAction{Op: "associate", Matter: id, Revision: m.Revision, Association: hqworkspace.Association{ID: "exact-worker", Conversation: stop.Conversation, Role: "execution", From: stop.OccurredAt}}, true, nil)
	m = read().Matters[id]
	var receipt hqworkspace.AssessmentReceipt
	handoff := workspaceAction{Op: "assess", ID: "verified-handoff", Matter: id, Revision: m.Revision, ObservationID: stop.ID, Generation: q.Generation, Decision: "handoff", ReviewRef: stop.Ref, Body: "Reviewed the incomplete worker stop and verified the original task."}
	r.action(t, pane, handoff, true, &receipt)
	d := read()
	if receipt.HandoffID != hqworkspace.GoalFollowupID(m) || d.Reconciliations[receipt.HandoffID].State != "pending" || len(d.IntentAttempts) != 0 {
		t.Fatal("verified handoff lost the goal or created authority", receipt)
	}
	seen, err := os.ReadFile(wakeReads)
	if err != nil || strings.Count(string(seen), workspaceContinuationWake) != 1 {
		t.Fatalf("expected exactly one workspace wake read by the HQ terminal, got %q: %v", seen, err)
	}
	t.Logf("worker %s; exact %s; verified owner %s; handoff %s; attempts %d", worker, stop.Ref, q.Owner, receipt.HandoffID, len(d.IntentAttempts))
}

func runTriageFixtureHQ(t *testing.T) {
	scan := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("❯ ")
		if !scan.Scan() {
			return
		}
		line := scan.Text()
		if strings.HasPrefix(line, "{") {
			var command workspaceFixtureCommand
			if err := json.Unmarshal([]byte(line), &command); err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(command.Action)
			if err != nil {
				t.Fatal(err)
			}
			code, raw := goalCLI(t, string(body), "hq", "workspace", "act", "--stdin", "--json")
			output, err := json.Marshal(workspaceFixtureResult{Code: code, Output: raw})
			if err != nil {
				t.Fatal(err)
			}
			workspaceReplayWrite(t, "cli-result-"+strconv.Itoa(command.ID), output)
		} else if strings.Contains(line, workspaceContinuationWake) {
			code, delta := goalCLI(t, "", "events", "--since-seq", "0", "--json")
			if code != 0 {
				t.Fatal(code, delta)
			}
			workspaceReplayWrite(t, "triage-cli-event-delta", []byte(delta))
			code, raw := goalCLI(t, "", "hq", "workspace", "reconcile", "--json")
			if code != 0 {
				t.Fatal(code, raw)
			}
			// The terminal's completed read is independent of a transport audit
			// arriving after the terminal has started handling the wake.
			f, err := os.OpenFile(filepath.Join(state.Home(), "triage-terminal-wakes"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintln(f, line)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			workspaceReplayWrite(t, "triage-cli-read", []byte(raw))
		}
	}
}
