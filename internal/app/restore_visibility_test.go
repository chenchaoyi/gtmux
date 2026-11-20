package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Both production senders resolve tmux.Bin to this log-only executable. Even a
// broken routing/occupation guard cannot reach the operator's default socket.
func visibleRestoreFixture(t *testing.T, uncertain bool, beforeSecond func()) (resume.Record, resume.Record, string) {
	t.Helper()
	plan, pane, _ := cnrAppPlan(t) // owns t.TempDir HOME/CODEX_HOME/XDG_DATA_HOME
	home := plan.Record.Cwd
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	a, b := plan.Record, plan.Record
	b.SessionID = "second-visible-thread"
	writeRecoveryFixture(t, b.SessionID, home)
	for loc, rec := range map[string]resume.Record{"pair:0.0": a, "pair:0.1": b} {
		if err := resume.Save(loc, rec); err != nil {
			t.Fatal(err)
		}
	}
	save := filepath.Join(home, ".tmux", "resurrect", "last")
	if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(save, []byte(savePaneCmd("pair", "0", "0", home, "codex", ":codex")+"\n"+savePaneCmd("pair", "0", "1", home, "codex", ":codex")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	calls, stub := filepath.Join(home, "calls"), filepath.Join(home, "tmux")
	script := `#!/bin/sh
shift
case "$1" in
 list-panes) printf '%%7\tbash\tpair:0.0\t%s\n%%8\tbash\tpair:0.1\t%s\n' "$HOME" "$HOME";;
 display-message) case "$*" in *%8*) printf '%%8\tpair:0.1\tbash\n';; *) printf '%%7\tpair:0.0\tbash\n';; esac;;
 send-keys) printf '%s\n' "$*" >> "$VISIBILITY_CALLS";;
esac
`
	if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldBin, oldFlag := tmux.Bin, restoreResumeFlag
	tmux.Bin = stub
	restoreResumeFlag = "auto"
	t.Setenv("VISIBILITY_CALLS", calls)
	t.Cleanup(func() { tmux.Bin = oldBin; restoreResumeFlag = oldFlag })
	checked := false
	nativeRestoreReadPane = func(id, loc string) (nativeRestorePaneState, bool) {
		if id == "%8" && !checked {
			checked = true
			if beforeSecond != nil {
				beforeSecond()
			}
		}
		current := *pane
		current.Pane = id
		current.Loc = loc
		return current, (id == "%7" && loc == "pair:0.0") || (id == "%8" && loc == "pair:0.1")
	}
	nativeRestoreSend = func(target, command string, enter bool) error {
		if tmux.Bin != stub {
			t.Fatal("sender isolation lost")
		}
		if err := tmux.SendText(target, command, enter); err != nil {
			return err
		}
		if uncertain && target == "%7" {
			return errors.New("synthetic uncertain Enter receipt")
		}
		return nil
	}
	return a, b, calls
}

func TestRRVLocalTimeoutVisibleBeforeNextPreparation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "stdout")
	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = file
	t.Cleanup(func() { os.Stdout = original; file.Close() })
	checked := false
	_, _, calls := visibleRestoreFixture(t, false, func() {
		checked = true
		body, err := os.ReadFile(out)
		if err != nil || !strings.Contains(string(body), "%7") || !strings.Contains(string(body), "Startup wait timed out") {
			t.Errorf("B prepared before visible A timeout: %q error=%v", body, err)
		}
	})
	resumeAgentsContext(context.Background())
	if !checked {
		t.Fatal("second preparation was not reached")
	}
	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(log), "send-keys -t %7 -l") != 1 || strings.Count(string(log), "send-keys -t %8 -l") != 1 || strings.Count(string(log), " Enter") != 2 {
		t.Fatal("sender target/Enter/replay contract", string(log))
	}
}

func TestRRVRemoteFinalStoreRetainsFirstIssueFact(t *testing.T) {
	for _, first := range []string{"pending", "unknown"} {
		t.Run(first, func(t *testing.T) {
			var store restorejob.Store
			var id string
			checked := false
			a, b, calls := visibleRestoreFixture(t, first == "unknown", func() {
				checked = true
				mid, err := store.Get(id)
				if err != nil {
					t.Fatal(err)
				}
				body, _ := json.Marshal(mid)
				var wire struct {
					Agents []map[string]any `json:"agents"`
				}
				if err := json.Unmarshal(body, &wire); err != nil {
					t.Fatal(err)
				}
				if len(wire.Agents) != 1 || wire.Agents[0]["state"] != "startup-timeout" || wire.Agents[0]["issue_state"] != first {
					t.Errorf("intermediate store lost first fact: %s", body)
				}
			})
			var rev string
			store, id, rev = remoteStoreAtCurrentHome(t)
			p := restorejob.Plan{Revision: rev, ResumeMode: "auto", Targets: map[string]restorejob.Target{"pair:0.0": {Agent: "codex", SessionID: a.SessionID, HomeID: a.HomeID}, "pair:0.1": {Agent: "codex", SessionID: b.SessionID, HomeID: b.HomeID}}, Sessions: []restorejob.Session{{Name: "pair", Agents: 2}}}
			reserveRemote(t, store, id, rev, p)
			runner := remoteRestoreRunner{Preview: func() (restorejob.Plan, error) { return p, nil }, Prepare: func() bool { return true }, Live: func() map[string]bool { return map[string]bool{"pair": true} }, Resume: func() { resumeAgentsContext(context.Background()) }}
			if code := runRemoteRestore(id, store, runner); code != 0 {
				t.Fatal(code)
			}
			if !checked {
				t.Fatal("second preparation was not reached")
			}
			// Fresh Store object forces the final persisted wire shape back through disk.
			job, err := (restorejob.Store{Now: store.Now}).Get(id)
			if err != nil || job.Phase != "partial" || len(job.Agents) != 2 {
				t.Fatal(job, err)
			}
			data, err := json.Marshal(job)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Agents []map[string]any `json:"agents"`
			}
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.Agents[0]["state"] != "startup-timeout" || wire.Agents[0]["issue_state"] != first || wire.Agents[1]["issue_state"] != "pending" {
				t.Fatalf("final receipt lost first issue fact: %s", data)
			}
			log, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(log), " -l ") != 2 || strings.Count(string(log), " Enter") != 2 {
				t.Fatal("worker replayed or lost Enter", string(log))
			}
		})
	}
}

func remoteStoreAtCurrentHome(t *testing.T) (restorejob.Store, string, string) {
	t.Helper()
	now := time.Unix(1800000000, 0)
	return restorejob.Store{Now: func() time.Time { return now }}, fmt.Sprintf("%d-visible-1234567890", now.Unix()), strings.Repeat("a", 64)
}
