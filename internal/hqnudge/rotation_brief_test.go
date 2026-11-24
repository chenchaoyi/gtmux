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
				Drain(bind.Pane)
				if !Pending() || calls != 0 || f.pastes != 0 || f.enters != 0 {
					t.Fatal("the unbound line crossed the successor reservation")
				}
				f.nano += int64(3 * time.Second) // the old line is rejected after the handoff window
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
			case "no native route":
				if !Pending() {
					t.Fatal("unavailable routing discarded a provably unsent briefing")
				}
				f.nano += int64(3 * time.Second)
				Drain(bind.Pane)
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
						return dispatch.Result{State: dispatch.StateRefusedWaiting, Evidence: "codex-frontend-unavailable", Submission: dispatch.SubmissionNotSubmitted}
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
			Drain(bind.Pane)
			Drain(bind.Pane)
			if calls != 1 || len(ordinary) != 0 {
				t.Fatal("briefing did not get its window alone", calls, ordinary)
			}
			if err := recoverNativeIntents(); err != nil {
				t.Fatal(err)
			}
			f.nano += int64(20 * time.Second)
			Drain(bind.Pane)
			Drain(bind.Pane)
			if calls != 1 || len(ordinary) != 1 || !strings.Contains(ordinary[0], tick) ||
				!strings.Contains(ordinary[0], relay) || strings.Contains(ordinary[0], "STARTUP BRIEFING") ||
				f.pastes != 0 || f.enters != 0 {
				t.Fatal("held wake replayed or consumed independent input", calls, ordinary, f.pastes, f.enters)
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
		"sentAt": switched - 1, "submittedAtNS": time.Unix(switched-1, 0).UnixNano(),
		"switchedAt": switched, "successor": b.Session, "targetStamp": b.Stamp,
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

func TestHRB005ReservedSuccessorPrecedesImmediateAndPeriodicWakes(t *testing.T) {
	for _, publishFirst := range []bool{false, true} {
		name := "before briefing publication"
		if publishFirst {
			name = "briefing already queued"
		}
		t.Run(name, func(t *testing.T) {
			bind, bound, f, _ := rotationBriefFixture(t)
			var inputs []string
			prod.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
				inputs = append(inputs, payload)
				return dispatch.Result{State: dispatch.StateQueued}, true
			}
			nativeWakeIO = func(string, ...string) dispatch.IO {
				return dispatch.IO{
					CaptureColor: func() string { return f.capture(bind.Pane) }, Sleep: func() {},
					SubmitNative: func(opts dispatch.Opts, payload string, hold func() string) dispatch.Result {
						if !opts.MatchesTarget(bind.Session, bind.Home, bind.Instance, bind.ClientPID) || hold() != "" {
							t.Fatal("briefing lost its original successor", opts)
						}
						inputs = append(inputs, payload)
						return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
							Request: "receipt", Session: bind.Session, Home: bind.Home, Turn: "turn",
						}}
					},
				}
			}
			if publishFirst {
				queueBoundBrief(t, bind)
			} else {
				now := f.base / int64(time.Second)
				writeBriefRotation(t, map[string]any{
					"pane": bind.Pane, "agent": bind.Agent, "retiring": "old", "input": "/new",
					"createdAt": now - 2, "sentAt": now - 1, "reserveBrief": true,
				})
			}
			const tick = "» gtmux·tick │ workspace reconciliation"
			const relay = "» gtmux·agent-relay │ protected relay"
			if ok, err := ReserveRelayAttention(relay, func() (bool, error) { return true, nil }); !ok || err != nil {
				t.Fatal(ok, err)
			}
			// Relay admission uses the real clock. Keep the fake drain clock
			// after that admission even if the fixture takes over a second.
			f.nano = max(f.nano+int64(time.Second), time.Now().UnixNano()-f.base)
			DeliverKeyedAt(bind.Pane, "workspace-reconciliation", tick, 0)
			Drain(bind.Pane)
			if !publishFirst {
				if len(inputs) != 0 || len(queuedNames(func(string) bool { return true })) != 2 {
					t.Fatal("an automatic wake crossed the publication gap", inputs)
				}
				writeBriefRotation(t, bound)
				DeliverRotationBrief(bind, hqwake.StartupBriefingEN)
			}
			if len(inputs) != 1 || !strings.HasPrefix(inputs[0], hqwake.StartupBriefingEN+" · #") || len(queuedNames(func(string) bool { return true })) != 2 {
				t.Fatal("briefing and independent wakes did not get separate opportunities", inputs)
			}
			if err := os.Remove(briefRotationPath()); err != nil {
				t.Fatal(err)
			}
			Drain(bind.Pane)
			Drain(bind.Pane)
			if len(inputs) != 2 || !strings.Contains(inputs[1], tick) || !strings.Contains(inputs[1], relay) ||
				f.pastes != 0 || f.enters != 0 {
				t.Fatal("normal wakes did not resume, or a briefing replayed", inputs, f.pastes, f.enters)
			}
		})
	}
}

func TestHRB005SentHandoffHoldsLegacyRepair(t *testing.T) {
	bind, request, f, _ := rotationBriefFixture(t)
	if err := os.Remove(briefRotationPath()); err != nil {
		t.Fatal(err)
	}
	f.swallow = true
	deliver(prod, bind.Pane, "» gtmux·waiting own stranded")
	if f.pastes != 1 || f.enters != 1 || len(f.sent) != 0 {
		t.Fatal("fixture did not strand an automatic draft", f)
	}
	now := f.base / int64(time.Second)
	request["sentAt"], request["switchedAt"], request["bootstrapAt"] = now-1, now, now
	request["submittedAtNS"] = time.Unix(now-1, 0).UnixNano()
	writeBriefRotation(t, request)
	f.swallow = false
	f.nano += int64(enterRepairBackoff)
	Drain(bind.Pane)
	if f.enters != 1 || f.pastes != 1 || len(f.sent) != 0 {
		t.Fatal("legacy repair sent input during the handoff", f)
	}
	if err := os.Remove(briefRotationPath()); err != nil {
		t.Fatal(err)
	}
	Drain(bind.Pane)
	if f.enters != 2 || len(f.sent) != 1 {
		t.Fatal("existing exact legacy repair did not resume", f)
	}
}

func TestHRB006ComposerWaitPreservesBriefing(t *testing.T) {
	for _, name := range []string{"active", "waiting", "draft", "copy mode", "unreadable", "late draft", "receiver busy"} {
		t.Run(name, func(t *testing.T) {
			bind, _, f, _ := rotationBriefFixture(t)
			calls, accepted := 0, 0
			busy := name == "active" || name == "waiting"
			prod.busy = func(string) bool { return busy }
			f.inMode, f.unstructured = name == "copy mode", name == "unreadable"
			if name == "draft" {
				f.draft = "original draft"
			}
			nativeWakeIO = func(string, ...string) dispatch.IO {
				return dispatch.IO{
					CaptureColor: func() string { return f.capture(bind.Pane) }, Sleep: func() {},
					SubmitNative: func(_ dispatch.Opts, _ string, hold func() string) dispatch.Result {
						calls++
						if calls == 1 && name == "late draft" {
							f.draft = "new human draft"
							if hold() == "" {
								t.Fatal("final guard missed new human draft")
							}
							return dispatch.Result{State: dispatch.StateRefusedDraft, Submission: dispatch.SubmissionNotSubmitted}
						}
						if calls == 1 && name == "receiver busy" {
							return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted, Evidence: "wake-loaded-idle-required"}
						}
						if hold() != "" {
							t.Fatal("briefing did not retain its safe destination")
						}
						accepted++
						return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
							Request: "receipt", Session: bind.Session, Home: bind.Home, Turn: "turn",
						}}
					},
				}
			}
			queueBoundBrief(t, bind)
			queued := queuedNames(isRotationBrief)
			Drain(bind.Pane)
			after := queuedNames(isRotationBrief)
			if len(queued) != 1 || len(after) != 1 || after[0] != queued[0] || accepted != 0 || f.pastes != 0 || f.enters != 0 {
				t.Fatalf("unsafe composer did not preserve the original briefing: %v %v %d %d", queued, after, calls, accepted)
			}
			if name == "draft" && f.draft != "original draft" || name == "late draft" && f.draft != "new human draft" {
				t.Fatal("human draft changed")
			}
			wantFirstCalls := 0
			if name == "late draft" || name == "receiver busy" {
				wantFirstCalls = 1
			}
			if calls != wantFirstCalls {
				t.Fatal("unexpected attempted dispatch in unsafe composer", calls)
			}
			busy, f.inMode, f.unstructured, f.draft = false, false, false, ""
			f.nano += int64(time.Second)
			Drain(bind.Pane)
			Drain(bind.Pane)
			if calls != wantFirstCalls+1 || accepted != 1 || Pending() || len(auditRecords(t, events.AuditEventWakeDelivered)) != 0 ||
				len(auditRecords(t, events.AuditEventWakeAccepted)) != 1 || f.pastes != 0 || f.enters != 0 {
				t.Fatal("safe reopening did not accept exactly once", calls, accepted)
			}
		})
	}
}
