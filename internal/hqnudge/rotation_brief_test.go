package hqnudge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHRB001RotationBriefBindsOriginalRecipient(t *testing.T) {
	for _, name := range []string{
		"correct", "expired", "deadline during native check", "new request", "removed request", "agent change",
		"successor change", "foreign home", "pane reuse", "frontend replacement", "late successor change",
		"copy mode", "draft", "unreadable composer", "no native route", "old unbound line",
	} {
		t.Run(name, func(t *testing.T) {
			bind, request, f, target := rotationBriefFixture(t)
			calls, inputs := 0, 0
			nativeWakeIO = func(string, ...string) dispatch.IO {
				if name == "no native route" {
					return dispatch.IO{}
				}
				return dispatch.IO{
					CaptureColor: func() string { return f.capture(bind.Pane) },
					Sleep:        func() {}, InMode: func() bool { return f.inMode },
					SubmitNative: func(opts dispatch.Opts, payload string, hold func() string) dispatch.Result {
						calls++
						end, hasDeadline := opts.Context.Deadline()
						if !opts.IdleWakeInput || opts.DirectUserInput || opts.ActiveImageInput ||
							!opts.MatchesTarget(bind.Session, bind.Home, bind.Instance, bind.ClientPID) ||
							opts.MatchesTarget("different", bind.Home, bind.Instance, bind.ClientPID) ||
							!hasDeadline || end.Unix() != bind.Deadline ||
							!strings.HasPrefix(payload, hqwake.StartupBriefingEN+" · #") {
							t.Fatal("briefing lost its addressed idle submission", opts, payload)
						}
						switch name {
						case "late successor change":
							target.Session = "successor-2"
						case "deadline during native check":
							f.nano += int64(3 * time.Second)
						}
						if hold() != "" {
							return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted}
						}
						inputs++
						return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
							Request: "synthetic-receipt", Session: bind.Session, Home: bind.Home, Turn: "synthetic-turn",
						}}
					},
					Paste: func(string) error { t.Fatal("briefing used native terminal paste"); return nil },
					Enter: func() error { t.Fatal("briefing used native terminal Enter"); return nil },
				}
			}
			if name == "old unbound line" {
				Enqueue(hqwake.StartupBriefingZH)
			} else {
				queueBoundBrief(t, bind)
			}
			switch name {
			case "expired":
				f.nano += int64(3 * time.Second)
			case "new request":
				request["bootstrapID"] = "other-request"
				writeBriefRotation(t, request)
			case "removed request":
				if err := os.Remove(briefRotationPath()); err != nil {
					t.Fatal(err)
				}
			case "agent change":
				request["agent"] = "claude"
				writeBriefRotation(t, request)
				*target = rotationBriefTarget{}
			case "successor change":
				target.Session = "successor-2"
			case "foreign home":
				target.Home = "foreign-home"
			case "pane reuse":
				target.Stamp = "10:20:31"
			case "frontend replacement":
				target.Instance = "different-instance"
			case "copy mode":
				f.inMode = true
			case "draft":
				f.draft = "the user's draft"
			case "unreadable composer":
				f.unstructured = true
			}
			Drain(bind.Pane)
			Drain(bind.Pane)
			wantCalls, wantInputs, wantRefused := 0, 0, 1
			switch name {
			case "correct":
				wantCalls, wantInputs, wantRefused = 1, 1, 0
			case "late successor change", "deadline during native check":
				wantCalls = 1
			case "draft", "copy mode", "unreadable composer":
				wantRefused = 0 // the shared outer guard leaves its original queue entry alone
				if !Pending() {
					t.Fatal("the existing input gate did not leave the entry pending")
				}
			}
			if calls != wantCalls || inputs != wantInputs || f.pastes != 0 || f.enters != 0 {
				t.Fatalf("native calls=%d (inputs=%d), paste=%d enter=%d", calls, inputs, f.pastes, f.enters)
			}
			if got := ReadHealth(time.Now().Unix()); got.Refused != wantRefused || got.Uncertain != 0 {
				t.Fatalf("unexpected final queue inventory: %+v", got)
			}
			if len(auditRecords(t, events.AuditEventWakeAccepted)) != wantInputs ||
				len(auditRecords(t, events.AuditEventWakeDelivered)) != 0 {
				t.Fatal("the receiver and independent delivery audits diverged")
			}
		})
	}
}

func TestHRB002RotationBriefNeverRetriesOrJoinsOrdinaryWake(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "positive refusal"
		if unknown {
			name = "possible submission"
		}
		t.Run(name, func(t *testing.T) {
			bind, _, f, _ := rotationBriefFixture(t)
			calls := 0
			nativeWakeIO = func(string, ...string) dispatch.IO {
				return dispatch.IO{
					CaptureColor: func() string { return f.capture(bind.Pane) }, Sleep: func() {},
					SubmitNative: func(_ dispatch.Opts, _ string, hold func() string) dispatch.Result {
						calls++
						if why := hold(); why != "" {
							t.Fatal(why)
						}
						if unknown {
							return dispatch.Result{State: dispatch.StateFailed, Submission: dispatch.SubmissionUnknown}
						}
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "wake-loaded-idle-required", Submission: dispatch.SubmissionNotSubmitted}
					},
				}
			}
			queueBoundBrief(t, bind)
			const tick = "» gtmux·tick │ workspace reconciliation"
			const relay = "» gtmux·agent-relay │ protected relay"
			EnqueueKeyed("workspace-reconciliation", tick)
			if ok, err := ReserveRelayAttention(relay, func() (bool, error) { return true, nil }); !ok || err != nil {
				t.Fatal(ok, err)
			}
			f.nano += int64(time.Second) // the reserved Relay used the real wall clock for admission
			var ordinary []string
			prod.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
				ordinary = append(ordinary, payload)
				return dispatch.Result{State: dispatch.StateQueued}, true
			}
			// The protected Relay and workspace reminder get their existing window.
			Drain(bind.Pane)
			if calls != 0 || len(ordinary) != 1 || !strings.Contains(ordinary[0], tick) ||
				!strings.Contains(ordinary[0], relay) || strings.Contains(ordinary[0], "STARTUP BRIEFING") {
				t.Fatal("independent wakes lost their input window", calls, ordinary)
			}
			Drain(bind.Pane)
			// A second independent workspace notice also drains after the bound attempt.
			f.nano += int64(20 * time.Second)
			EnqueueKeyed("workspace-reconciliation", tick)
			Drain(bind.Pane)
			Drain(bind.Pane)
			if calls != 1 || len(ordinary) != 2 || f.pastes != 0 || f.enters != 0 {
				t.Fatal("briefing replayed or interrupted independent input", calls, ordinary, f.pastes, f.enters)
			}
			got := ReadHealth(time.Now().Unix())
			if unknown {
				if got.Uncertain != 1 || got.Refused != 0 {
					t.Fatalf("unknown generation did not remain protected: %+v", got)
				}
				records, err := os.ReadDir(nativeIntentDir())
				if err != nil || len(records) != 1 {
					t.Fatal("missing original protected intent", err, records)
				}
				data, err := os.ReadFile(filepath.Join(nativeIntentDir(), records[0].Name()))
				var intent nativeIntent
				if err != nil || json.Unmarshal(data, &intent) != nil || intent.Outcome != "uncertain" ||
					intent.Retry.Number != 0 || !strings.HasPrefix(intent.Payload, hqwake.StartupBriefingEN+" · #") {
					t.Fatal("protected intent lost its original identity or gained a retry", err, intent)
				}
			} else if got.Refused != 1 || got.Uncertain != 0 {
				t.Fatalf("definite refusal did not remain distinct: %+v", got)
			}
		})
	}
}

func rotationBriefFixture(t *testing.T) (hqinput.RotationBrief, map[string]any, *fake, *rotationBriefTarget) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	now := time.Now().Truncate(time.Second)
	f := &fake{base: now.UnixNano()}
	target := &rotationBriefTarget{Home: "synthetic-home", Session: "successor-1", Stamp: "10:20:30", Instance: "frontend-1", PID: 33}
	b := hqinput.RotationBrief{Request: "original-request", Pane: "%isolated", Agent: "codex",
		Home: target.Home, Session: target.Session, Stamp: target.Stamp, Instance: target.Instance,
		ClientPID: target.PID, Deadline: now.Add(3 * time.Second).Unix()}
	switched := b.Deadline - int64(hqinput.RotationReadyWait/time.Second)
	request := map[string]any{
		"pane": b.Pane, "agent": b.Agent, "retiring": "old-session", "input": "/new", "createdAt": switched - 2,
		"sentAt": switched - 1, "switchedAt": switched, "successor": b.Session, "targetStamp": b.Stamp,
		"bootstrapAt": switched, "bootstrapID": b.Request, "bootstrapHome": b.Home, "bootstrapInstance": b.Instance, "bootstrapPID": b.ClientPID,
	}
	writeBriefRotation(t, request)
	savedProd, savedIO, savedTarget, savedIdentity := prod, nativeWakeIO, nativeWakeTarget, nativeWakeIdentity
	t.Cleanup(func() {
		prod, nativeWakeIO, nativeWakeTarget, nativeWakeIdentity = savedProd, savedIO, savedTarget, savedIdentity
	})
	prod = f.io()
	prod.rotationBriefTarget = func(string) rotationBriefTarget { return *target }
	nativeWakeTarget, nativeWakeIdentity = func() string { return b.Pane }, prod.identity
	return b, request, f, target
}

func briefRotationPath() string {
	return filepath.Join(state.Dir(), "hqwake", "rotate-pending.json")
}

func writeBriefRotation(t *testing.T, request map[string]any) {
	t.Helper()
	b, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(briefRotationPath(), b); err != nil {
		t.Fatal(err)
	}
}

func queueBoundBrief(t *testing.T, bind hqinput.RotationBrief) {
	t.Helper()
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal(err, acquired)
	}
	defer release()
	DeliverRotationBrief(bind, hqwake.StartupBriefingEN)
	if !Pending() {
		t.Fatal("briefing did not enter the durable queue while input was held")
	}
}
