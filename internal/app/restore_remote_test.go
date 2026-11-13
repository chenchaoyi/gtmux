package app

import (
	"errors"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func remoteFixture(t *testing.T) (restorejob.Store, string, string) {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	now := time.Unix(1800000000, 0)
	return restorejob.Store{Now: func() time.Time { return now }}, fmt.Sprintf("%d-request-1234567890", now.Unix()), strings.Repeat("a", 64)
}
func TestRemotePreviewOnlyMissingAndBindsProducerInputs(t *testing.T) {
	remoteFixture(t)
	save := filepath.Join(t.TempDir(), "saved.txt")
	body := saveShellPane("missing", "0", "0", "/tmp") + "\n" + saveShellPane("detached", "0", "0", "/tmp") + "\n" + saveShellPane("attached", "0", "0", "/tmp") + "\n"
	if e := os.WriteFile(save, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	query := func(...string) (string, error) { return "0\t@1\t%1\tdetached\n1\t@2\t%2\tattached\n", nil }
	p, e := remoteRestorePlanWith(save, query, time.Unix(1800000000, 0))
	if e != nil || len(p.Sessions) != 1 || p.Sessions[0].Name != "missing" {
		t.Fatal(p, e)
	}
	if p.InputStamp == "" || p.Revision == "" || p.SavedAt == 0 {
		t.Fatal("preview lacks source evidence", p)
	}
	r := resume.Record{Agent: "codex", SessionID: "saved", Cwd: "/tmp", HomeID: "synthetic", UpdatedAt: 1}
	if e := resume.Save("missing:0.0", r); e != nil {
		t.Fatal(e)
	}
	p2, e := remoteRestorePlanWith(save, query, time.Now())
	if e != nil || p2.Revision == p.Revision {
		t.Fatal("record mutation not bound", e)
	}
	if _, e := remoteRestorePlanWith(save, func(...string) (string, error) { return "", errors.New("unreadable") }, time.Now()); e == nil {
		t.Fatal("unreadable topology accepted")
	}
	if e := os.WriteFile(save, []byte(body+"# save changed\n"), 0600); e != nil {
		t.Fatal(e)
	}
	p3, e := remoteRestorePlanWith(save, query, time.Now())
	if e != nil || p3.Revision == p2.Revision {
		t.Fatal("save bytes not bound", e)
	}
}
func reserveRemote(t *testing.T, s restorejob.Store, id, rev string, p restorejob.Plan) {
	t.Helper()
	if _, e := s.Start(id, rev, func() (restorejob.Plan, error) { return p, nil }, func(string) error { return nil }); e != nil {
		t.Fatal(e)
	}
}
func TestRemoteWorkerRejectsStaleAndBusyBeforeMutating(t *testing.T) {
	for _, which := range []string{"stale", "busy", "verify", "layout"} {
		t.Run(which, func(t *testing.T) {
			s, id, rev := remoteFixture(t)
			p := restorejob.Plan{Revision: rev, ResumeMode: "auto", Sessions: []restorejob.Session{{Name: "missing", Agents: 1}}}
			reserveRemote(t, s, id, rev, p)
			prepared, resumed := 0, 0
			runner := remoteRestoreRunner{Preview: func() (restorejob.Plan, error) {
				if which == "stale" {
					p.Revision = strings.Repeat("b", 64)
				}
				return p, nil
			}, Prepare: func() bool {
				prepared++
				if restoreResumeFlag != "off" {
					t.Error("prepare allowed automatic startup before verification")
				}
				return which != "layout"
			}, Live: func() map[string]bool { return map[string]bool{"missing": true} }, Resume: func() { resumed++ }, Verify: func(restorejob.Plan) error {
				if which == "verify" {
					return errors.New("save changed")
				}
				return nil
			}}
			if which == "busy" {
				l, e := restoreExecutionLease()
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
			}
			if got := runRemoteRestore(id, s, runner); got != 1 {
				t.Fatal(got)
			}
			if resumed != 0 {
				t.Fatal("failed worker sent agent input")
			}
			if (which == "stale" || which == "busy") && prepared != 0 {
				t.Fatal("mutated before guard")
			}
			j, e := s.Get(id)
			if e != nil || j.Phase != "failed" {
				t.Fatal(j, e)
			}
			if got := runRemoteRestore(id, s, runner); got != 1 {
				t.Fatal("duplicate worker ran")
			}
		})
	}
}
func TestRemoteWorkerKeepsPartialAndConfirmedResultsSeparate(t *testing.T) {
	s, id, rev := remoteFixture(t)
	p := restorejob.Plan{Revision: rev, ResumeMode: "auto", Sessions: []restorejob.Session{{Name: "missing", Agents: 2}, {Name: "other", Agents: 0}}}
	reserveRemote(t, s, id, rev, p)
	n := 0
	runner := remoteRestoreRunner{Preview: func() (restorejob.Plan, error) { return p, nil }, Prepare: func() bool {
		if !remoteRestoreSessions["missing"] || remoteRestoreSessions["existing"] {
			t.Fatal("scope widened")
		}
		return true
	}, Live: func() map[string]bool { return map[string]bool{"missing": true} }, Resume: func() {
		n++
		reportRemoteRestore("agents", []resumeAttempt{{Loc: "missing:0.0", Pane: "%7", State: "confirmed"}, {Loc: "missing:0.1", Pane: "%8", State: "pending"}})
	}}
	if got := runRemoteRestore(id, s, runner); got != 0 {
		t.Fatal(got)
	}
	j, e := s.Get(id)
	if e != nil || j.Phase != "partial" || j.Sessions[1].State != "missing" || j.Agents[1].State != "pending" || n != 1 {
		t.Fatal(j, e, n)
	}
	if remoteRestoreSessions != nil || remoteRestoreProgress != nil {
		t.Fatal("worker state leaked")
	}
}
func TestRemotePaneGuardRechecksExactShellWithoutTouchingOthers(t *testing.T) {
	remoteFixture(t)
	bin := filepath.Join(t.TempDir(), "tmux")
	if e := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$FIXTURE_PANE\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	old := tmux.Bin
	tmux.Bin = bin
	remoteRestoreSessions = map[string]bool{"missing": true}
	defer func() { tmux.Bin = old; remoteRestoreSessions = nil }()
	for _, v := range []struct {
		row string
		ok  bool
	}{{"%7\tmissing:0.0\tbash", true}, {"%7\tmissing:0.0\tcodex", false}, {"%8\tmissing:0.0\tbash", false}, {"%7\tother:0.0\tbash", false}, {"", false}} {
		t.Setenv("FIXTURE_PANE", v.row)
		if got := remotePaneStillEligible("%7", "missing:0.0"); got != v.ok {
			t.Fatal(v, got)
		}
	}
}
func TestRestoreExecutionLeaseIsProcessOwnedAndFailsClosed(t *testing.T) {
	remoteFixture(t)
	l, e := restoreExecutionLease()
	if e != nil {
		t.Fatal(e)
	}
	if other, e := restoreExecutionLease(); e == nil {
		other.Close()
		t.Fatal("parallel restore admitted")
	}
	l.Close()
	l, e = restoreExecutionLease()
	if e != nil {
		t.Fatal(e)
	}
	l.Close()
	if e := os.RemoveAll(filepath.Join(state.Dir(), "lease-restore-execution.lock")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(state.Dir(), "lease-restore-execution.lock"), 0700); e != nil {
		t.Fatal(e)
	}
	if l, e = restoreExecutionLease(); e == nil {
		l.Close()
		t.Fatal("unsafe lock target admitted")
	}
}

func TestRemoteAgentProducerKeepsApprovedIdentityAndSkipsOtherSessions(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			remoteFixture(t)
			home := state.Home()
			save := filepath.Join(home, ".tmux", "resurrect", "last")
			if e := os.MkdirAll(filepath.Dir(save), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(save, []byte(savePaneCmd("missing", "0", "0", home, "codex", ":codex")+"\n"+savePaneCmd("existing", "0", "0", home, "codex", ":codex")+"\n"), 0600); e != nil {
				t.Fatal(e)
			}
			for _, s := range []string{"missing", "existing"} {
				writeRecoveryFixture(t, s+"-thread", home)
				if e := resume.Save(s+":0.0", resume.Record{Agent: "codex", SessionID: s + "-thread", Cwd: home}); e != nil {
					t.Fatal(e)
				}
			}
			bin := filepath.Join(home, "tmux-fixture")
			log := filepath.Join(home, "commands")
			script := `#!/bin/sh
shift
case "$1" in
 list-panes) printf '%%7\tbash\tmissing:0.0\t%s\n%%8\tbash\texisting:0.0\t%s\n' "$HOME" "$HOME";;
 display-message) printf '%%7\tmissing:0.0\tbash\n';;
 send-keys) printf '%s\n' "$*" >> "$FIXTURE_CALLS";;
esac
`
			if e := os.WriteFile(bin, []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			t.Setenv("FIXTURE_CALLS", log)
			oldBin, oldFlag := tmux.Bin, restoreResumeFlag
			tmux.Bin = bin
			restoreResumeFlag = "type"
			remoteRestoreSessions = map[string]bool{"missing": true}
			remoteRestoreTargets = map[string]restorejob.Target{"missing:0.0": {Agent: "codex", SessionID: "missing-thread"}}
			if changed {
				remoteRestoreTargets["missing:0.0"] = restorejob.Target{Agent: "codex", SessionID: "different-thread"}
			}
			defer func() {
				tmux.Bin = oldBin
				restoreResumeFlag = oldFlag
				remoteRestoreSessions = nil
				remoteRestoreTargets = nil
				remoteRestoreProgress = nil
				remoteRestoreError = nil
			}()
			resumeAgents()
			b, e := os.ReadFile(log)
			if changed {
				if e == nil && len(b) > 0 {
					t.Fatal("replaced target received startup", string(b))
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			out := string(b)
			if !strings.Contains(out, "send-keys -t %7 -l") || strings.Contains(out, "%8") || strings.Contains(out, " Enter") {
				t.Fatal("startup scope/mode changed", out)
			}
		})
	}
}

func TestRemotePreviewRejectsLargeSaveBeforeHistoryOrTopology(t *testing.T) {
	remoteFixture(t)
	save := filepath.Join(t.TempDir(), "save")
	var b strings.Builder
	for i := 0; i < 257; i++ {
		b.WriteString(saveShellPane(fmt.Sprintf("session-%d", i), "0", "0", "/tmp") + "\n")
	}
	if e := os.WriteFile(save, []byte(b.String()), 0600); e != nil {
		t.Fatal(e)
	}
	_, e := remoteRestorePlanWith(save, func(...string) (string, error) {
		t.Fatal("oversize preview reached topology/history resolution")
		return "", nil
	}, time.Now())
	if e == nil || e.Error() != "plan_too_large" {
		t.Fatal(e)
	}
}
