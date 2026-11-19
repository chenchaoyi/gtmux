package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"golang.org/x/term"
)

const (
	replayMatter      = "cli-review-matter"
	replayStep        = "write-proof"
	replayAction      = "write-proof-once"
	replayRef         = "message:fixture-authority"
	replayInstruction = "Create the reversible fixture marker once."
	replayHome        = "fixture-worker-home"
	replayWorker      = "fixture-worker-session"
)

type workspaceCLIReplay struct {
	home, exe string
	commands  int
}

func registerWorkspaceReplayProbe() {
	workspaceProbeOnce.Do(func() { hqnudge.RegisterRevalidator(workspaceReconciliationProbe) })
}

func newWorkspaceCLIReplay(t *testing.T) *workspaceCLIReplay {
	t.Helper()
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.MkdirTemp("/tmp", "hq-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	for _, key := range []string{"TMUX", "TMUX_PANE", "CODEX_THREAD_ID", "CODEX_SESSION_ID"} {
		t.Setenv(key, "")
	}
	config := filepath.Join(home, "tmux.conf")
	if err := os.WriteFile(config, []byte("set -g remain-on-exit on\n"), 0600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(home, "tmux")
	script := "#!/bin/sh\nexec " + shellQuote(bin) + " -S " + shellQuote(filepath.Join(home, "sock")) + " -f " + shellQuote(config) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldBin := tmux.Bin
	tmux.Bin = wrapper
	t.Cleanup(func() { _ = tmux.OK("kill-server"); tmux.Bin = oldBin })
	if err := state.MkdirAll(state.HQHome(), 0700); err != nil {
		t.Fatal(err)
	}
	registerWorkspaceReplayProbe()
	return &workspaceCLIReplay{home: home, exe: exe}
}

func (r *workspaceCLIReplay) pane(t *testing.T, name, role, mode string) string {
	t.Helper()
	dir := r.home
	if role == "hq" {
		dir = state.HQHome()
	}
	pane, err := tmux.Run("new-session", "-d", "-x", "180", "-y", "35", "-s", name, "-c", dir,
		"-P", "-F", "#{pane_id}", "-e", "HOME="+r.home, "-e", "GTMUX_WORKSPACE_CLI_ROLE="+role,
		"-e", "GTMUX_WORKSPACE_CLI_MODE="+mode,
		"exec "+shellQuote(r.exe)+" -test.run=^TestWorkspaceContinuationCLI$")
	if err != nil {
		t.Fatal(err, pane)
	}
	if err := state.WriteMarker(state.ActivePath(pane), name); err != nil {
		t.Fatal(err)
	}
	waitWorkspaceFixture(t, pane, func() bool { return strings.Contains(tmux.CapturePane(pane), "❯") })
	return pane
}

func waitWorkspaceFixture(t *testing.T, pane string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(18 * time.Second)
	for !ready() {
		if time.Now().After(deadline) || tmux.Display(pane, "#{pane_dead}") == "1" {
			t.Fatalf("fixture did not finish in pane %s: %s", pane, tmux.CapturePane(pane))
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func workspaceFixtureAction(t *testing.T, a workspaceAction, out any) {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	code, raw := goalCLI(t, string(b), "hq", "workspace", "act", "--stdin", "--json")
	if code != 0 {
		t.Fatalf("CLI %s exited %d: %s", a.Op, code, raw)
	}
	if out != nil && json.Unmarshal([]byte(raw), out) != nil {
		t.Fatalf("invalid CLI receipt: %s", raw)
	}
}

type workspaceFixtureCommand struct {
	ID     int             `json:"id"`
	Action workspaceAction `json:"action"`
}

type workspaceFixtureResult struct {
	Code   int    `json:"code"`
	Output string `json:"output"`
}

func (r *workspaceCLIReplay) action(t *testing.T, pane string, a workspaceAction, expectOK bool, out any) {
	t.Helper()
	r.commands++
	b, err := json.Marshal(workspaceFixtureCommand{ID: r.commands, Action: a})
	if err != nil {
		t.Fatal(err)
	}
	if !tmux.OK("send-keys", "-t", pane, "-l", string(b)) || !tmux.OK("send-keys", "-t", pane, "Enter") {
		t.Fatal("could not submit test-only CLI request")
	}
	path := filepath.Join(r.home, "cli-result-"+strconv.Itoa(r.commands))
	var result workspaceFixtureResult
	waitWorkspaceFixture(t, pane, func() bool {
		b, e := os.ReadFile(path)
		return e == nil && json.Unmarshal(b, &result) == nil
	})
	if (result.Code == 0) != expectOK || (expectOK && out != nil && json.Unmarshal([]byte(result.Output), out) != nil) {
		t.Fatalf("CLI action %s: expected success=%v, code=%d, output=%s", a.Op, expectOK, result.Code, result.Output)
	}
}

func workspaceReplayWrite(t *testing.T, name string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), name), contents, 0600); err != nil {
		t.Fatal(err)
	}
}

func workspaceCLIReplayHQ(t *testing.T) {
	// Substitute only discovery of the fixture agent process. The ordinary
	// adapter still checks the real tmux locator and current resume/active IDs.
	workspaceGoalCandidates = func() []workspaceCandidate {
		pane := strings.TrimSpace(tmux.Display("fixture-worker", "#{pane_id}"))
		loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
		c, ok := workspaceConversation(radar.Pane{PaneID: pane, Loc: loc, Agent: "Claude Code"})
		if !ok {
			return nil
		}
		return []workspaceCandidate{{Conversation: c, Status: "idle"}}
	}
	input := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("❯ ")
		if !input.Scan() {
			return
		}
		line := input.Text()
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
			continue
		}
		if !strings.Contains(line, "workspace reconciliation") {
			t.Fatalf("unexpected HQ fixture input %q", line)
		}
		code, raw := goalCLI(t, "", "hq", "workspace", "reconcile", "--json")
		if code != 0 {
			t.Fatal(code, raw)
		}
		workspaceReplayWrite(t, "hq-read-"+os.Getenv("TMUX_PANE"), []byte(raw))
	}
}

func workspaceCLIReplayWorker(t *testing.T) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Restore(fd, old)
	const pasteStart = "\x1b[200~"
	const pasteEnd = "\x1b[201~"
	fmt.Print("\x1b[?2004h❯ ")
	input := bufio.NewReader(os.Stdin)
	for {
		prefix, err := input.Peek(len(pasteStart))
		if err != nil {
			return
		}
		if string(prefix) != pasteStart {
			_, _ = input.ReadByte()
			continue
		}
		_, _ = input.Discard(len(pasteStart))
		content, err := input.ReadString('\x1b')
		if err != nil {
			t.Fatal(err)
		}
		suffix := make([]byte, len(pasteEnd)-1)
		for i := range suffix {
			suffix[i], err = input.ReadByte()
			if err != nil {
				t.Fatal(err)
			}
		}
		if string(suffix) != pasteEnd[1:] {
			t.Fatal("unexpected bracketed-paste terminator")
		}
		payload := strings.ReplaceAll(strings.TrimSuffix(content, "\x1b"), "\r", "\n")
		fmt.Print(strings.ReplaceAll(payload, "\n", "\r\n"))
		key, err := input.ReadByte()
		if err != nil || key != '\r' {
			t.Fatalf("expected guarded Enter, key %q: %v", key, err)
		}
		want := "[gtmux goal-step " + replayStep + "; action " + replayAction + "; ref " + replayRef + "]\n" + replayInstruction
		if payload != want {
			t.Fatalf("fixture received unexpected payload %q", payload)
		}
		path := filepath.Join(os.Getenv("HOME"), "worker-effect")
		proof, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("fixture action ran more than once", err)
		}
		if _, err := proof.WriteString("performed " + replayAction + "\n"); err != nil {
			t.Fatal(err)
		}
		if err := proof.Close(); err != nil {
			t.Fatal(err)
		}
		fmt.Print("\x1b[3J\x1b[2J\x1b[H❯ ")
		pane := os.Getenv("TMUX_PANE")
		if os.Getenv("GTMUX_WORKSPACE_CLI_MODE") == "unknown" {
			// Simulate loss of both observation channels after the real Enter.
			// The fixture's own effect stays on disk. Its private tmux history
			// can otherwise confirm the submission even without a hook receipt.
			waitWorkspaceFixture(t, pane, func() bool {
				return !strings.Contains(tmux.CapturePane(pane), replayAction)
			})
			if !tmux.OK("clear-history", "-t", pane) {
				t.Fatal("fixture could not hide its lost terminal receipt")
			}
			continue
		}
		now := time.Now().Unix()
		if err := events.AppendChecked(events.Record{Ts: now, Event: "UserPromptSubmit", Pane: pane,
			Agent: "Claude Code", HomeID: replayHome, AgentSession: replayWorker, Summary: dispatch.NormalizeNeedle(payload)}); err != nil {
			t.Fatal(err)
		}
		receipt := `{"schema":"gtmux.goal-drive/v1","matter":"` + replayMatter + `","drive":{"step_id":"` + replayStep + `","action_id":"` + replayAction + `","ref":"` + replayRef + `","state":"progress","evidence":"fixture:worker-effect","next_check":` + strconv.FormatInt(now+300, 10) + `}}`
		// The worker fixture supplies its enrolled producer identity and writes
		// through the real relay store. Ingestion validates it against the saved intent.
		_, _, err = relay.Put(relay.Request{ID: "fixture-progress", Kind: "report", For: "hq", Agent: "claude",
			HomeID: replayHome, ThreadID: replayWorker, Pane: pane, Session: "fixture-worker", Body: receipt})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkspaceDraftKeepsBothWakeQueues(t *testing.T) {
	r := newWorkspaceCLIReplay(t)
	pane := r.pane(t, "fixture-hq", "hq", "draft")
	if !tmux.OK("send-keys", "-t", pane, "-l", "user draft between two matters") {
		t.Fatal("draft setup")
	}
	waitWorkspaceFixture(t, pane, func() bool {
		return strings.Contains(tmux.CapturePane(pane), "❯ user draft between two matters")
	})
	s, now := workspaceStore(), time.Now().Unix()
	standing, err := s.Create("standing-matter", "Standing matter", now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := notifyWorkspaceReconciliation(s, now, 0)
	if err != nil || next <= now || !hqnudge.Pending() {
		t.Fatal("standing wake did not queue", next, err)
	}
	fresh, err := s.Create("urgent-matter", "Urgent matter", now)
	if err != nil {
		t.Fatal(err)
	}
	o := hqworkspace.Observation{ID: "new-formal-review", Kind: "relay-report", MatterID: fresh.ID,
		Ref: "relay:new-formal-review", OccurredAt: now + 1, Continuation: true}
	if err := s.Ingest("fixture", 0, 1, []hqworkspace.Observation{o}, now+1); err != nil {
		t.Fatal(err)
	}
	if after, err := notifyWorkspaceReconciliation(s, now+1, next); err != nil || after != next {
		t.Fatal("urgent wake spent the standing interval", after, next, err)
	}
	hq.DrainHQNudges()
	if !strings.Contains(tmux.CapturePane(pane), "❯ user draft between two matters") {
		t.Fatal("a wake changed the live draft")
	}
	read, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(fresh.ID, "", "completed", "Fixture verified only the urgent matter", read.Matters[fresh.ID].Revision, now+2); err != nil {
		t.Fatal(err)
	}
	if !tmux.OK("send-keys", "-t", pane, "C-u") {
		t.Fatal("clear fixture draft")
	}
	waitWorkspaceFixture(t, pane, func() bool { return !strings.Contains(tmux.CapturePane(pane), "❯ user draft between two matters") })
	hq.DrainHQNudges()
	var snapshot struct {
		Pending []hqworkspace.Reconciliation `json:"pending"`
	}
	waitWorkspaceFixture(t, pane, func() bool {
		b, err := os.ReadFile(filepath.Join(r.home, "hq-read-"+pane))
		return err == nil && json.Unmarshal(b, &snapshot) == nil
	})
	if !containsWorkspaceFixtureObservation(snapshot.Pending, hqworkspace.GoalFollowupID(standing), "") {
		t.Fatal("the other matter did not survive the draft", snapshot)
	}
	journal, _ := events.ReadSince(0)
	var woke string
	for _, row := range journal {
		if row.Event == events.AuditEventWakeDelivered && row.Pane == pane {
			woke = row.Summary
		}
	}
	if woke == "" || strings.Contains(woke, workspaceContinuationWake) {
		t.Fatal("drain did not deliver just the still-relevant standing wake", woke)
	}
	t.Logf("draft retained both wake slots; verified standing delivery for %s: %s", standing.ID, woke)
}

func containsWorkspaceFixtureObservation(rows []hqworkspace.Reconciliation, id, body string) bool {
	for _, q := range rows {
		if q.Observation.ID == id && strings.Contains(q.Observation.Body, body) {
			return true
		}
	}
	return false
}

func TestWorkspaceContinuationCLI(t *testing.T) {
	switch os.Getenv("GTMUX_WORKSPACE_CLI_ROLE") {
	case "hq":
		workspaceCLIReplayHQ(t)
		return
	case "worker":
		workspaceCLIReplayWorker(t)
		return
	}
	for _, mode := range []string{"drive", "rotate", "worker-replaced", "unknown", "external-wait"} {
		t.Run(mode, func(t *testing.T) {
			r := newWorkspaceCLIReplay(t)
			pane := r.pane(t, "fixture-hq-old", "hq", mode)
			worker := r.pane(t, "fixture-worker", "worker", mode)
			hqpane.Stamp(pane)
			s, now := workspaceStore(), time.Now().Unix()
			if _, err := s.Create(replayMatter, "Authorized fixture goal", now-10); err != nil {
				t.Fatal(err)
			}
			loc := tmux.Display(worker, "#{session_name}:#{window_index}.#{pane_index}")
			if err := resume.Save(loc, resume.Record{Agent: "claude", HomeID: replayHome, SessionID: replayWorker, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if err := state.WriteMarker(state.ActivePath(worker), replayWorker); err != nil {
				t.Fatal(err)
			}
			c := hqworkspace.Conversation{Agent: "claude", HomeID: replayHome, SessionID: replayWorker}
			reviewer := hqworkspace.Conversation{Agent: "claude", HomeID: "fixture-review-home", SessionID: "fixture-reviewer"}
			for _, a := range []hqworkspace.Association{
				{ID: "bound-worker", Conversation: c, Role: "execution", From: now - 10},
				{ID: "bound-reviewer", Conversation: reviewer, Role: "execution", From: now - 10},
			} {
				if err := s.Associate(replayMatter, a, 0, now-10); err != nil {
					t.Fatal(err)
				}
			}
			workspaceFixtureAction(t, workspaceAction{Op: "send", ID: "fixture-authority", Matter: replayMatter, Body: replayInstruction}, nil)
			if err := s.ActivateObservations(events.CurrentSeq(), now-9); err != nil {
				t.Fatal(err)
			}
			const evidence = "The reversible marker is the only remaining work."
			_, _, err := relay.Put(relay.Request{ID: "fixture-formal-review", Kind: "report", For: "hq", Agent: reviewer.Agent,
				HomeID: reviewer.HomeID, ThreadID: reviewer.SessionID, Pane: worker, Session: "fixture-reviewer", Body: evidence})
			if err != nil {
				t.Fatal(err)
			}
			scan := func() {
				t.Helper()
				if err := ingestWorkspaceObservations(s, time.Now().Unix()); err != nil {
					t.Fatal(err)
				}
			}
			read := func() hqworkspace.Database {
				t.Helper()
				db, err := s.Read()
				if err != nil {
					t.Fatal(err)
				}
				return db
			}
			scan()
			if _, err := notifyWorkspaceReconciliation(s, time.Now().Unix(), now+300); err != nil {
				t.Fatal(err)
			}
			hq.DrainHQNudges()
			const sourceID = "relay-fixture-formal-review"
			var snapshot struct {
				Pending []hqworkspace.Reconciliation `json:"pending"`
			}
			waitWorkspaceFixture(t, pane, func() bool {
				b, err := os.ReadFile(filepath.Join(r.home, "hq-read-"+pane))
				return err == nil && json.Unmarshal(b, &snapshot) == nil
			})
			if !containsWorkspaceFixtureObservation(snapshot.Pending, sourceID, evidence) {
				t.Fatal("HQ CLI did not read fresh source evidence", snapshot)
			}
			claim := workspaceAction{Op: "claim", ObservationID: sourceID}
			var source hqworkspace.Reconciliation
			r.action(t, pane, claim, true, &source)
			db := read()
			if source.Owner != "::fixture-hq-old" || len(db.IntentAttempts) != 0 ||
				hqworkspace.GoalResponsibility(db, db.Matters[replayMatter], time.Now().Unix()).State != "verify" ||
				tmux.Display(worker, "#{pane_dead}") == "1" {
				t.Fatal("claim alone changed the source or goal", source)
			}
			if _, err := os.Stat(filepath.Join(r.home, "worker-effect")); !os.IsNotExist(err) {
				t.Fatal("claim alone reached the worker", err)
			}
			if keep, _ := workspaceReconciliationProbe(workspaceContinuationWake); keep {
				t.Fatal("an exact live claim still produces an urgent wake")
			}
			t.Logf("claim-only: %s by %s generation %d; zero action attempts and no worker effect", sourceID, source.Owner, source.Generation)
			note := workspaceAction{Op: "assess", ID: "fixture-noted", ObservationID: sourceID, Matter: replayMatter,
				Generation: source.Generation, Revision: db.Matters[replayMatter].Revision, Decision: "noted", Body: "HQ read the exact formal reviewer receipt"}
			if mode == "rotate" {
				oldPane, oldGeneration := pane, source.Generation
				pane = r.pane(t, "fixture-hq-new", "hq", mode)
				hqpane.Unstamp(oldPane)
				hqpane.Stamp(pane)
				r.action(t, pane, claim, false, nil)
				r.action(t, oldPane, note, false, nil)
				// Advance the one private fixture lease across its expiry without
				// sleeping five minutes or changing an actual HQ's clock.
				row := db.Reconciliations[sourceID]
				row.LeaseUntil = time.Now().Unix() - 1
				db.Reconciliations[sourceID] = row
				raw, err := json.Marshal(db)
				if err != nil {
					t.Fatal(err)
				}
				if err := state.WritePrivateFile(filepath.Join(state.HQHome(), "workspace", "records.json"), raw); err != nil {
					t.Fatal(err)
				}
				if keep, _ := workspaceReconciliationProbe(workspaceContinuationWake); !keep {
					t.Fatal("unadvanced expired evidence did not return to HQ")
				}
				r.action(t, pane, claim, true, &source)
				if source.Owner != "::fixture-hq-new" || source.Generation <= oldGeneration {
					t.Fatal("successor did not claim the exact evidence", source)
				}
				r.action(t, pane, note, false, nil)
				t.Logf("HQ claim handoff: %s generation %d -> %s generation %d", oldPane, oldGeneration, source.Owner, source.Generation)
				note.Generation = source.Generation
			}
			r.action(t, pane, note, true, nil)
			m := read().Matters[replayMatter]
			var goal hqworkspace.Reconciliation
			r.action(t, pane, workspaceAction{Op: "claim", ObservationID: hqworkspace.GoalFollowupID(m)}, true, &goal)
			p := &hqworkspace.Responsibility{StepID: replayStep, Action: replayInstruction, AuthorityRef: replayRef,
				Intent: &hqworkspace.ActionIntent{ID: replayAction, Ref: replayRef, Target: c, At: time.Now().Unix()}}
			if mode == "external-wait" {
				p.Intent = nil
				p.Hold = &hqworkspace.GoalHold{Kind: "external", SourceRef: "fixture:pending-check",
					Condition: "Wait for the independently verified fixture check", NextCheck: time.Now().Unix() + 300}
			}
			r.action(t, pane, workspaceAction{Op: "assess", ID: "fixture-responsibility", ObservationID: goal.Observation.ID,
				Matter: replayMatter, Revision: m.Revision, Generation: goal.Generation, Decision: "responsibility",
				Body: "Reviewer receipt and pre-authorized fixture instruction checked", Responsibility: p}, true, nil)
			db = read()
			m = db.Matters[replayMatter]
			if db.Messages["fixture-authority"].Body != replayInstruction || m.Responsibility == nil ||
				m.Responsibility.AuthorityRef != replayRef || len(db.IntentAttempts) != 0 {
				t.Fatal("next responsibility and authority were not recorded separately", m.Responsibility)
			}
			t.Logf("durable next responsibility: assessment fixture-responsibility, step %s, authority %s", m.Responsibility.StepID, m.Responsibility.AuthorityRef)
			if mode == "worker-replaced" {
				if err := state.WriteMarker(state.ActivePath(worker), "fixture-replacement-session"); err != nil {
					t.Fatal(err)
				}
			}
			drive := workspaceAction{Op: "drive", ID: replayAction, Matter: replayMatter, Revision: m.Revision}
			if mode == "external-wait" {
				r.action(t, pane, drive, false, nil)
				if d := read(); len(d.IntentAttempts) != 0 || hqworkspace.GoalResponsibility(d, d.Matters[replayMatter], time.Now().Unix()).State != "held" {
					t.Fatal("external hold yielded a worker attempt")
				}
				if _, err := os.Stat(filepath.Join(r.home, "worker-effect")); !os.IsNotExist(err) {
					t.Fatal("held work reached the worker", err)
				}
				t.Log("sourced external wait stayed held with zero attempts")
				return
			}
			var attempt, repeat hqworkspace.IntentAttempt
			r.action(t, pane, drive, true, &attempt)
			r.action(t, pane, drive, true, &repeat)
			if attempt.State != repeat.State || attempt.At != repeat.At || !attempt.Intent.Target.Same(c) || len(read().IntentAttempts) != 1 {
				t.Fatal("retry changed the exact action or worker", attempt, repeat)
			}
			effect, err := os.ReadFile(filepath.Join(r.home, "worker-effect"))
			switch mode {
			case "worker-replaced":
				if attempt.State != "held" || !os.IsNotExist(err) {
					t.Fatal("reused pane received input", attempt, err)
				}
				t.Logf("original worker identity fenced replacement window: %s", attempt.Evidence)
			case "unknown":
				if attempt.State != "uncertain" || err != nil || string(effect) != "performed "+replayAction+"\n" {
					t.Fatal("unknown outcome was treated as a retryable or known action", attempt, err)
				}
				if _, err := relay.Get("fixture-progress"); err != relay.ErrMissing {
					t.Fatal("unknown case unexpectedly had a relay progress receipt")
				}
				t.Logf("worker effect exists with no receipt; repeat preserved %s attempt", attempt.State)
			default:
				if attempt.State != "landed" || err != nil || string(effect) != "performed "+replayAction+"\n" {
					t.Fatal("authorized worker did not run the exact submitted step", attempt, err)
				}
				waitWorkspaceFixture(t, worker, func() bool {
					_, err := relay.Get("fixture-progress")
					return err == nil
				})
				scan()
				db := read()
				m = db.Matters[replayMatter]
				out := hqworkspace.GoalResponsibility(db, m, time.Now().Unix())
				proof := db.Reconciliations[out.ObservationID].Observation
				if out.State != "driving" || proof.ID != "relay-fixture-progress" || !proof.Conversation.Same(c) ||
					proof.DriveProof == nil || proof.DriveProof.Evidence != "fixture:worker-effect" {
					t.Fatal("source work was not independently attributed to its exact intent", out, proof)
				}
				journal, _ := events.ReadSince(0)
				submitted := 0
				for _, row := range journal {
					if row.Event == "UserPromptSubmit" && row.Pane == worker && row.AgentSession == c.SessionID && row.HomeID == c.HomeID {
						submitted++
					}
				}
				if submitted != 1 {
					t.Fatal("wanted one exact worker submission", submitted)
				}
				t.Logf("one submit to worker %s/%s/%s at pane %s; action %s %s, independent %s proof %s (%s)",
					c.Agent, c.HomeID, c.SessionID, worker, attempt.Intent.ID, attempt.State, proof.DriveProof.State, proof.ID, proof.DriveProof.Evidence)
			}
		})
	}
}
