package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// These hooks exist only inside the one detached restore worker. They are never
// installed by serve; HTTP goroutines cannot mutate restore's process globals.
var remoteRestoreSessions map[string]bool
var remoteRestoreTargets map[string]restorejob.Target
var remoteRestoreProgress func(string, []resumeAttempt) error
var remoteRestoreError error

func reportRemoteRestore(phase string, attempts []resumeAttempt) {
	if remoteRestoreProgress != nil && remoteRestoreError == nil {
		remoteRestoreError = remoteRestoreProgress(phase, attempts)
	}
}

func remoteRestorePlan() (restorejob.Plan, error) {
	save := resurrectLastSave()
	p, e := remoteRestorePlanWith(save, tmux.Run, time.Now())
	if e == nil && save != resurrectLastSave() {
		e = &restorejob.Error{Code: "plan_changed"}
	}
	return p, e
}
func remoteRestorePlanWith(save string, query func(...string) (string, error), now time.Time) (restorejob.Plan, error) {
	p := restorejob.Plan{ServerTime: now.Unix(), Sessions: []restorejob.Session{}, ResumeMode: []string{"auto", "type", "off"}[effectiveResumeMode()]}
	before, e := readRestoreSave(save)
	if e != nil {
		return p, e
	}
	// Reject excessive work before resolving histories for the preview.
	sessionNames := map[string]bool{}
	panes := 0
	for _, line := range strings.Split(string(before), "\n") {
		if sp, ok := parseSavedPaneLine(line); ok {
			sessionNames[sp.Session] = true
			panes++
			if len(sessionNames) > restorejob.MaxSessions || panes > restorejob.MaxAgents {
				return p, &restorejob.Error{Code: "plan_too_large"}
			}
		}
	}
	plan, e := pendingRestorePlan(save, query)
	if e != nil {
		return p, e
	}
	for _, s := range plan.Sessions {
		if s.State != "missing" {
			continue
		}
		row := restorejob.Session{Name: s.Name, Windows: s.Windows, Panes: s.Panes, Agents: len(s.Agents), State: "missing"}
		for _, a := range s.Agents {
			if a.Alive {
				row.Resumable++
			}
		}
		p.Sessions = append(p.Sessions, row)
	}
	if len(p.Sessions) > restorejob.MaxSessions {
		return p, &restorejob.Error{Code: "plan_too_large"}
	}
	if save != "" {
		if st, e := os.Stat(save); e == nil {
			p.SavedAt = st.ModTime().Unix()
		}
	}
	// Bind full producer inputs, including resume metadata omitted by the public
	// summary. Do not put private paths, commands, goals or thread IDs in the API.
	p.Targets = map[string]restorejob.Target{}
	for _, session := range plan.Sessions {
		if session.State != "missing" {
			continue
		}
		for _, a := range session.Agents {
			p.Targets[a.Loc] = restorejob.Target{Agent: a.Agent, SessionID: a.SessionID}
		}
	}
	wanted := map[string]bool{}
	for _, s := range p.Sessions {
		wanted[s.Name] = true
	}
	var records []resume.Located
	for _, r := range resume.AllLocated() {
		if wanted[locSession(r.Loc)] {
			records = append(records, r)
			if target, ok := p.Targets[r.Loc]; ok && target.SessionID == r.SessionID && target.Agent == r.Agent {
				target.HomeID = r.HomeID
				p.Targets[r.Loc] = target
			}
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Loc < records[j].Loc })
	after, e := readRestoreSave(save)
	if e != nil || string(before) != string(after) {
		return p, &restorejob.Error{Code: "plan_changed"}
	}
	private := struct {
		Save    string
		Bytes   []byte
		Plan    restorePlan
		Records []resume.Located
		Mode    string
	}{save, before, plan, records, p.ResumeMode}
	stamp, e := remoteRestoreInputs(save, wanted, p.ResumeMode)
	if e != nil {
		return p, e
	}
	p.InputStamp, p.SavePath = stamp, save
	b, e := json.Marshal(private)
	if e != nil {
		return p, e
	}
	p.Revision = fmt.Sprintf("%x", sha256.Sum256(append(b, []byte(stamp)...)))
	return p, nil
}
func remoteRestoreInputs(save string, wanted map[string]bool, mode string) (string, error) {
	b, e := readRestoreSave(save)
	if e != nil {
		return "", e
	}
	var rows []resume.Located
	for _, r := range resume.AllLocated() {
		if wanted[locSession(r.Loc)] {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Loc < rows[j].Loc })
	all, e := json.Marshal(struct {
		Save    string
		Bytes   []byte
		Records []resume.Located
		Mode    string
	}{save, b, rows, mode})
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", sha256.Sum256(all)), nil
}
func remotePaneStillEligible(id, loc string) bool {
	if remoteRestoreSessions == nil {
		return true
	}
	out, e := tmux.Run("display-message", "-p", "-t", id, "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_current_command}")
	f := strings.Split(strings.TrimSpace(out), "\t")
	return e == nil && len(f) == 3 && f[0] == id && f[1] == loc && remoteRestoreSessions[locSession(loc)] && isShellCommand(f[2])
}
func readRestoreSave(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	f, e := state.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("restore save unavailable")
	}
	b, e := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if e != nil || len(b) > 8<<20 {
		return nil, fmt.Errorf("restore save exceeds budget")
	}
	return b, nil
}
func restorePreviewSnapshot() (restorejob.Snapshot, error) {
	p, e := remoteRestorePlan()
	if e != nil {
		return restorejob.Snapshot{}, e
	}
	j, e := (restorejob.Store{}).Latest()
	return restorejob.Snapshot{Plan: p, Job: j}, e
}
func startRemoteRestore(id, revision string) (*restorejob.Job, error) {
	return (restorejob.Store{}).Start(id, revision, remoteRestorePlan, launchRestoreWorker)
}
func launchRestoreWorker(id string) error {
	c := exec.Command(selfPath(), "restore", "--run-job="+id)
	c.Dir = state.Home()
	c.Env = os.Environ()
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// No inherited terminal or response pipe; a closed App/HTTP request is not a
	// reason to cancel an explicitly accepted restore. Never log saved commands.
	if e := c.Start(); e != nil {
		return e
	}
	go func() { _ = c.Wait() }()
	return nil
}
func restoreExecutionLease() (*state.Lease, error) { return state.TryLease("restore-execution") }

type remoteRestoreRunner struct {
	Preview func() (restorejob.Plan, error)
	Prepare func() bool
	Live    func() map[string]bool
	Resume  func()
	Verify  func(restorejob.Plan) error
}

func runRemoteRestoreJob(id string) int {
	// A hard worker process budget. If it fires, the released kernel lease lets
	// read-only status classify interruption; the same ID can never be re-claimed.
	timer := time.AfterFunc(restorejob.WorkerBudget, func() { os.Exit(124) })
	defer timer.Stop()
	return runRemoteRestore(id, restorejob.Store{}, remoteRestoreRunner{Preview: remoteRestorePlan, Prepare: ensureServerResult, Live: liveSessionNames, Resume: resumeAgents, Verify: func(p restorejob.Plan) error {
		if p.SavePath != resurrectLastSave() {
			return fmt.Errorf("save changed")
		}
		stamp, e := remoteRestoreInputs(p.SavePath, remoteRestoreSessions, p.ResumeMode)
		if e != nil || stamp != p.InputStamp {
			return fmt.Errorf("saved inputs changed")
		}
		return nil
	}})
}
func runRemoteRestore(id string, store restorejob.Store, runner remoteRestoreRunner) int {
	j, worker, e := store.Claim(id)
	if e != nil {
		return 1
	}
	defer worker.Close()
	fail := func(code string) int { _ = store.Update(id, "failed", code, nil, nil); return 1 }
	lease, e := restoreExecutionLease()
	if e != nil {
		return fail("restore_busy")
	}
	defer lease.Close()
	p, e := runner.Preview()
	if e != nil || p.Revision != j.Revision {
		return fail("plan_changed")
	}
	remoteRestoreTargets = p.Targets
	remoteRestoreSessions = map[string]bool{}
	for _, s := range p.Sessions {
		remoteRestoreSessions[s.Name] = true
	}
	oldFromApp, oldFlag := restoreFromApp, restoreResumeFlag
	restoreFromApp = true
	restoreResumeFlag = p.ResumeMode
	var results []restorejob.Result
	remoteRestoreProgress = func(phase string, attempts []resumeAttempt) error {
		if attempts != nil {
			results = make([]restorejob.Result, 0, len(attempts))
			for _, a := range attempts {
				results = append(results, restorejob.Result{Loc: a.Loc, Pane: a.Pane, State: a.State})
			}
		}
		return store.Update(id, phase, "", nil, results)
	}
	defer func() {
		remoteRestoreTargets = nil
		remoteRestoreSessions = nil
		remoteRestoreProgress = nil
		remoteRestoreError = nil
		restoreFromApp = oldFromApp
		restoreResumeFlag = oldFlag
	}()
	reportRemoteRestore("layout", nil)
	if remoteRestoreError != nil {
		return 1
	}
	// Restore layout first. Existing ensureServer must not start agents before
	// the job can verify its approved inputs again.
	restoreResumeFlag = "off"
	if !runner.Prepare() {
		return fail("layout_failed")
	}
	if remoteRestoreError != nil {
		return 1
	}
	restoreResumeFlag = p.ResumeMode
	if runner.Verify != nil {
		if e := runner.Verify(p); e != nil {
			return fail("saved_inputs_changed")
		}
	}
	if runner.Resume != nil {
		runner.Resume()
	}
	if remoteRestoreError != nil {
		return 1
	}
	live := runner.Live()
	rows := append([]restorejob.Session(nil), p.Sessions...)
	partial := false
	for i := range rows {
		rows[i].State = "missing"
		if live[rows[i].Name] {
			rows[i].State = "restored"
		} else {
			partial = true
		}
	}
	for _, a := range results {
		if a.State != "confirmed" {
			partial = true
		}
	}
	expected := 0
	for _, s := range rows {
		expected += s.Agents
	}
	if p.ResumeMode != "off" && len(results) < expected {
		partial = true
	}
	phase, code := "completed", ""
	if partial {
		phase = "partial"
		code = "check_results"
	}
	if e := store.Update(id, phase, code, rows, results); e != nil {
		return 1
	}
	return 0
}
