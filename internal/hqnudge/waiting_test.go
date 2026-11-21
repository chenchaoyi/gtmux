package hqnudge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type waitingFixture struct {
	rows     map[string]string
	frames   map[string]string
	owners   map[string]waitingConversation
	readable bool
}

func newWaitingFixture(t *testing.T) *waitingFixture {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	f := &waitingFixture{
		rows: map[string]string{}, frames: map[string]string{},
		owners: map[string]waitingConversation{}, readable: true,
	}
	oldTmux, oldOwner, oldTask, oldCapture := waitingTmux, waitingOwner, waitingTask, waitingCapture
	t.Cleanup(func() {
		waitingTmux, waitingOwner, waitingTask, waitingCapture = oldTmux, oldOwner, oldTask, oldCapture
	})
	waitingTmux = func(args ...string) (string, error) {
		if args[0] == "list-panes" && f.readable {
			var rows []string
			for _, row := range f.rows {
				rows = append(rows, row)
			}
			return strings.Join(rows, "\n"), nil
		}
		if args[0] == "display-message" && f.rows[args[3]] != "" {
			return f.rows[args[3]], nil
		}
		return "", errors.New("terminal temporarily unavailable")
	}
	waitingOwner = func(p waitingTerminal) waitingConversation { return f.owners[p.Pane] }
	waitingTask = func(pane string) (dispatch.Task, bool) {
		return dispatch.Task{ID: "task-" + pane[1:], Pane: pane, Agent: "codex", Session: "synthetic-worker", CreatedAt: 1700000001}, true
	}
	waitingCapture = func(pane string) string { return f.frames[pane] }
	f.setPane("%90", "900") // a surviving pane on the original server
	return f
}

func (f *waitingFixture) setPane(pane, root string) {
	loc := "synthetic-worker:" + pane[1:] + ".0"
	f.rows[pane] = strings.Join([]string{"/tmp/synthetic-tmux", "321", "1700000000", pane, root, "synthetic-worker", loc, "0", "0"}, "\t")
	f.owners[pane] = waitingConversation{"codex", transcript.CodexHomeID(), "conversation-" + pane, "frontend-" + pane}
	f.frames[pane] = "╭──────────────╮\n│ › original draft │\n╰──────────────╯"
}

func queueDraftWait(t *testing.T, f *waitingFixture, pane string) string {
	t.Helper()
	f.setPane(pane, pane[1:]+"00")
	if err := state.WriteMarker(state.WaitingPath(pane), "draft"); err != nil {
		t.Fatal(err)
	}
	line := hqwake.Line(hqwake.ClassWaiting, "synthetic-worker:0.0 ("+pane+")", "stuck before running — draft")
	src := WaitingSourceForPane(pane)
	if src.Task == "" {
		t.Fatal("fixture could not verify a waiting source")
	}
	DeliverWaiting("", line, src, "draft", "", "", "", "")
	names := queuedNames(func(n string) bool { return strings.HasSuffix(n, waitingWakeSuffix) })
	for _, name := range names {
		b, err := readNativeBytes(filepath.Join(queueDir(), name))
		var w waitingWake
		if err == nil && json.Unmarshal(b, &w) == nil && w.Line == line && w.Source == src && w.Round > 0 {
			return line
		}
	}
	t.Fatal("producer did not retain the exact source and round")
	return ""
}

func TestHWW001FreshWaitingRevalidationWithProtectedRelay(t *testing.T) {
	f := newWaitingFixture(t)
	active := queueDraftWait(t, f, "%51")
	retired := queueDraftWait(t, f, "%52")
	resolved := queueDraftWait(t, f, "%53")
	unknown := queueDraftWait(t, f, "%54")
	reused := queueDraftWait(t, f, "%55")
	otherHome := queueDraftWait(t, f, "%56")
	newer := queueDraftWait(t, f, "%57")
	f.frames["%53"] = "╭──────────────╮\n│ ›              │\n╰──────────────╯"
	f.frames["%54"] = "unstructured frame without an agent composer"
	f.setPane("%55", "75555")
	f.frames["%55"] = f.frames["%53"]
	f.frames["%57"] = f.frames["%53"]
	delete(f.rows, "%52")
	delete(f.rows, "%56")
	if err := os.Chtimes(state.WaitingPath("%57"), time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	// Rewrite only the Home of this fixture's queued identity. A positive
	// absence on this tmux server must not be applied to that foreign record.
	for _, name := range queuedNames(func(n string) bool { return strings.HasSuffix(n, waitingWakeSuffix) }) {
		path := filepath.Join(queueDir(), name)
		b, err := readNativeBytes(path)
		if err != nil {
			t.Fatal(err)
		}
		var w waitingWake
		if err := json.Unmarshal(b, &w); err != nil {
			t.Fatal(err)
		}
		if w.Line == otherHome {
			w.Source.Owner.Home = "another-home"
			body, err := json.Marshal(w)
			if err != nil {
				t.Fatal(err)
			}
			if err := state.WritePrivateFile(path, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	hq := &fake{base: time.Now().Add(time.Minute).UnixNano(), draft: "the commander's unsent draft"}
	x := hq.io()
	legacy := hqwake.Line(hqwake.ClassWaiting, "older-worker:0.0 (%66)", "stuck before running — draft")
	enqueue(x, legacy)
	relay := hqwake.Line(hqwake.ClassTick, "workspace relay follow-up", "relay:synthetic-first-window", "pull: gtmux relay show synthetic-first-window")
	if ok, err := ReserveRelayAttention(relay, func() (bool, error) { return true, nil }); !ok || err != nil {
		t.Fatal("could not reserve the new relay", err)
	}
	var sent []string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		sent = append(sent, payload)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	drain(x, "%16")
	if len(sent) != 0 || hq.draft != "the commander's unsent draft" || len(auditRecords(t, events.AuditEventWakeDropped)) != 0 {
		t.Fatal("the draft guard was bypassed")
	}
	hq.draft = ""
	drain(x, "%16")
	drain(x, "%16")
	if len(sent) != 1 || strings.Count(sent[0], relay) != 1 || hq.pastes != 0 || hq.enters != 0 {
		t.Fatalf("the first eligible native window must carry the new relay once: %q", sent)
	}
	for i, line := range []string{active, unknown, reused, otherHome, newer, legacy} {
		if strings.Count(sent[0], line) != 1 {
			t.Fatalf("preserved waiting line %d is missing: %q", i, sent[0])
		}
	}
	if strings.Contains(sent[0], retired) || strings.Contains(sent[0], resolved) {
		t.Fatal("a proved obsolete wait was forwarded")
	}
	drops := auditRecords(t, events.AuditEventWakeDropped)
	if len(drops) != 2 ||
		drops[0].Outcome != events.DropWaitSourceRetired || drops[0].TaskID != "task-52" || drops[0].AgentSession != "conversation-%52" ||
		drops[1].Outcome != events.DropWaitResolved || drops[1].TaskID != "task-53" {
		t.Fatalf("want two source-attributed drop records, got %#v", drops)
	}
}

func TestHWW002PreviouslyAttemptedWaitingKeepsRecoveryRules(t *testing.T) {
	for _, mode := range []string{"retry", "unknown", "accepted"} {
		t.Run(mode, func(t *testing.T) {
			f := newWaitingFixture(t)
			line := queueDraftWait(t, f, "%61")
			hq := &fake{base: time.Now().Add(time.Minute).UnixNano()}
			x := hq.io()
			calls := 0
			x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
				calls++
				if !strings.Contains(payload, line) {
					t.Fatal("original generation was changed")
				}
				switch {
				case mode == "accepted":
					return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
						Request: "existing-receipt", Home: transcript.CodexHomeID(), Turn: "old-turn", Session: "hq-session",
					}}, true
				case mode == "retry" && calls == 1:
					return dispatch.Result{State: dispatch.StateRefusedDraft, Submission: dispatch.SubmissionNotSubmitted,
						FailureCode: dispatch.InputDraftPresent}, true
				}
				return dispatch.Result{State: dispatch.StateFailed, Submission: dispatch.SubmissionUnknown}, true
			}
			drain(x, "%16")
			if calls != 1 {
				t.Fatalf("expected a real first attempt, got %d", calls)
			}
			delete(f.rows, "%61")
			hq.nano += int64(6 * time.Second)
			drain(x, "%16")
			drain(x, "%16")
			want := 1
			if mode == "retry" {
				want++
			}
			if calls != want || len(auditRecords(t, events.AuditEventWakeDropped)) != 0 {
				t.Fatalf("retirement interfered with protection: calls=%d want=%d", calls, want)
			}
			if mode == "accepted" {
				files, err := os.ReadDir(acceptedWakeDir())
				if err != nil || len(files) != 1 {
					t.Fatal("accepted receipt was lost", err, files)
				}
			} else {
				items := readQueue()
				if len(items) != 1 || !strings.HasSuffix(items[0].Name(), waitingWakeSuffix+nativeUncertainSuffix) {
					t.Fatal("uncertain waiting generation was not retained", items)
				}
			}
		})
	}
}
