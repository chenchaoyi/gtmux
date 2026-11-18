package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestRRBFinalPauseConfirmsBeforeNextStart(t *testing.T) {
	g, clock := serialSourceFixture(context.Background(), resumeAuto, 30)
	began := *clock
	var events []string
	g.observe = func(a resumeAttempt) bool {
		if clock.Sub(began) >= 20*time.Millisecond {
			events = append(events, "ready-"+a.Pane)
			return true
		}
		return false
	}
	a, _ := g.run(func() (resumeAttempt, bool) {
		events = append(events, "start-A")
		return resumeAttempt{Pane: "A", State: "pending"}, true
	})
	if a.State != "confirmed" || clock.Sub(began) != 50*time.Millisecond {
		t.Fatalf("last pause lost ready: %+v events=%v elapsed=%s", a, events, clock.Sub(began))
	}
	_, _ = g.run(func() (resumeAttempt, bool) {
		if strings.Join(events, ",") != "start-A,ready-A" {
			t.Fatal(events)
		}
		return resumeAttempt{Pane: "B", State: "missing"}, false
	})
}

func TestRRBSelectionChangeDuringQueueRefusesSecond(t *testing.T) {
	for _, kind := range []string{"archive", "owner", "unchanged"} {
		t.Run(kind, func(t *testing.T) {
			plan, pane, _ := cnrAppPlan(t)
			save := resurrectLastSave()
			if save == "" {
				save = filepath.Join(os.Getenv("HOME"), ".tmux", "resurrect", "last")
				if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(save, []byte(savePaneCmd("worker", "0", "0", plan.Record.Cwd, "codex", ":codex resume cnr-session")+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			layout := loadSavedLayout(resurrectLastSave())
			selected := &nativeRestoreSelection{Path: layout.SavePath, Digest: layout.Digest, ReadError: layout.ReadError, Incumbent: plan.Incumbent}
			g, _ := serialSourceFixture(context.Background(), resumeAuto, 2)
			changed := false
			g.observe = func(resumeAttempt) bool { return changed }
			g.pause = func(context.Context, time.Duration) bool {
				changed = true
				if kind == "archive" {
					if err := os.WriteFile(save, []byte(saveShellPane("worker", "0", "0", plan.Record.Cwd)+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "owner" {
					rec := plan.Record
					rec.SessionID = "replacement"
					if err := resume.Save(pane.Loc, rec); err != nil {
						t.Fatal(err)
					}
				}
				return true
			}
			g.run(func() (resumeAttempt, bool) { return resumeAttempt{Pane: "first", State: "pending"}, true })
			sends := 0
			nativeRestoreSend = func(string, string, bool) error { sends++; return nil }
			a, _, sent := tryNativeRestoreContext(context.Background(), pane.Pane, pane.Loc, plan.Record, map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{}, selected)
			if kind == "unchanged" {
				if !sent || sends != 1 {
					t.Fatal(a, sent, sends)
				}
			} else if sent || sends != 0 || a.State != "target-changed" {
				t.Fatal(a, sent, sends)
			}
		})
	}
}

func TestRRBUnknownRemainsOccupiedAcrossCompatibilityRoute(t *testing.T) {
	plan, _, _ := cnrAppPlan(t)
	calls := filepath.Join(t.TempDir(), "calls")
	stub := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nif [ \"$2\" = display-message ]; then printf '%%7\\tworker:0.0\\tbash\\n'; exit 0; fi\nprintf '%s\\n' \"$*\" >> \"$RRB_CALLS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	oldBin := tmux.Bin
	tmux.Bin = stub
	t.Cleanup(func() { tmux.Bin = oldBin })
	t.Setenv("RRB_CALLS", calls)
	boot, err := resume.NativeRestoreBootID()
	if err != nil {
		t.Fatal(err)
	}
	nativeRestoreBoot = func() (string, error) { return boot, nil }
	plan.Identity.BootID = boot
	nativeRestoreReserve = resume.ReserveNativeRestoreAttempt
	nativeRestoreUpdate = resume.UpdateNativeRestoreAttempt
	sends := 0
	nativeRestoreSend = func(string, string, bool) error { sends++; return errors.New("uncertain Enter") }
	a, _ := routeRestoreCommand(context.Background(), plan.Pane.Pane, plan.Pane.Loc, plan.Record, map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{}, nil)
	if a.State != "unknown" || sends != 1 {
		t.Fatal(a, sends)
	}
	rec := plan.Record
	rec.Launcher = "company-wrapper"
	a, sent := routeRestoreCommand(context.Background(), plan.Pane.Pane, plan.Pane.Loc, rec, map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{}, nil)
	log, _ := os.ReadFile(calls)
	if sent || a.State != "already-attempted" || sends != 1 || len(log) != 0 {
		t.Fatal("compatibility replayed", a, sent, sends)
	}
	rec.SessionID = "fresh"
	a, sent = routeRestoreCommand(context.Background(), plan.Pane.Pane, plan.Pane.Loc, rec, map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{}, nil)
	log, _ = os.ReadFile(calls)
	if !sent || a.State != "pending" || !strings.Contains(string(log), "send-keys -t %7 -l") || !strings.Contains(string(log), "company-wrapper resume 'fresh'") || !strings.Contains(string(log), "send-keys -t %7 Enter") {
		t.Fatal("fresh wrapper contract", a, sent, string(log))
	}
	occupied, err := resume.NativeRestoreOccupied(resume.Record{Agent: "codex", SessionID: "fresh", HomeID: rec.HomeID})
	if err != nil || occupied {
		t.Fatal("fresh wrapper blocked", occupied, err)
	}
}

// Compatibility fixtures state their executable route; host Codex installation
// cannot turn an old bridge assertion into an assertion about a native recipe.
func rrbCompatibleExecutable(t *testing.T) {
	t.Helper()
	old := nativeRestoreInspectBinary
	nativeRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) { return nativeRestoreBinary{}, false, nil }
	t.Cleanup(func() { nativeRestoreInspectBinary = old })
	t.Setenv("CODEX_EXEC_SERVER_URL", "")
}
func rrbWaitShell(t *testing.T, loc string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if isShellCommand(tmux.Display(loc, "#{pane_current_command}")) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("private shell never started at %s", loc)
		}
	}
}

func TestRRBNativeEntryUsesFullPaneEvidenceAndExactTargets(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			plan, pane, binary := cnrAppPlan(t) // HOME/CODEX_HOME are assigned from t.TempDir.
			home := plan.Record.Cwd
			if os.Getenv("HOME") != home || !(strings.HasPrefix(home, "/private/tmp/") || strings.HasPrefix(home, "/tmp/") || strings.HasPrefix(home, os.TempDir()+string(os.PathSeparator))) {
				t.Fatal("fixture home is not private", home)
			}
			save := filepath.Join(home, ".tmux", "resurrect", "last")
			if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
				t.Fatal(err)
			}
			body := savePaneCmd("worker", "0", "0", home, "codex", ":codex") + "\n" + savePaneCmd("other", "0", "0", home, "codex", ":codex") + "\n"
			if err := os.WriteFile(save, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(home, "calls")
			stub := filepath.Join(home, "tmux")
			script := `#!/bin/sh
shift
case "$1" in
 list-panes) printf '%%7\tbash\tworker:0.0\t%s\n%%8\tbash\tother:0.0\t%s\n' "$HOME" "$HOME";;
 display-message)
  case "$*" in
   *pane_pid*) printf '%%7\tworker:0.0\t101\t/dev/ttys001\t%s\tbash\n' "$HOME";;
   *) printf '%%7\tworker:0.0\tbash\n';;
  esac;;
 send-keys) printf '%s\n' "$*" >> "$RRB_CALLS";;
esac
`
			if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			oldBin, oldFlag, oldProc := tmux.Bin, restoreResumeFlag, nativeRestoreReadProcess
			tmux.Bin = stub
			restoreResumeFlag = "type"
			t.Setenv("RRB_CALLS", calls)
			t.Cleanup(func() { tmux.Bin = oldBin; restoreResumeFlag = oldFlag; nativeRestoreReadProcess = oldProc })
			nativeRestoreReadPane = readNativeRestorePane
			nativeRestoreReadProcess = func(pid int) (launchhost.AgentProcess, bool) {
				return launchhost.AgentProcess{PID: pid, Comm: "bash", Start: pane.RootBirth}, pid == 101
			}
			// SendText is real; its executable is the private log-only fixture above.
			nativeRestoreSend = tmux.SendText
			remoteRestoreSessions = map[string]bool{"worker": true}
			remoteRestoreTargets = map[string]restorejob.Target{"worker:0.0": {Agent: "codex", SessionID: plan.Record.SessionID, HomeID: plan.Record.HomeID}}
			if changed {
				remoteRestoreTargets["worker:0.0"] = restorejob.Target{Agent: "codex", SessionID: "replacement"}
			}
			resumeAgentsContext(context.Background())
			log, err := os.ReadFile(calls)
			if changed {
				if err == nil && len(log) > 0 {
					t.Fatal("changed target sent", string(log))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, want, err := resume.NativeRestoreCommand(plan.Record, binary.Physical, filepath.Join(home, ".codex"), true, "")
			if err != nil {
				t.Fatal(err)
			}
			output := string(log)
			if !strings.Contains(output, "send-keys -t %7 -l "+want) || strings.Contains(output, "%8") || strings.Contains(output, " Enter") || strings.Contains(output, " codex -- ") {
				t.Fatal("native entry routing changed", output, want)
			}
		})
	}
}
