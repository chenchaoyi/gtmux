package hqnudge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

func TestHRB003RotationBriefJournalReadableAfterRestart(t *testing.T) {
	for _, outcome := range []string{"prepared", "uncertain", "accepted"} {
		t.Run(outcome, func(t *testing.T) {
			original := seedRotationBriefRecovery(t, t.TempDir(), outcome)
			if !strings.HasSuffix(original.Entries[0].Name, ".txt.sending") {
				t.Errorf("the v2 intent has a claim name the previous reader cannot read: %s", original.Entries[0].Name)
			}
			verifyRotationBriefRecovery(t, original)
		})
	}
}

func TestHRB004LoneRelayReportsUnavailableTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{base: time.Now().UnixNano()}
	x := f.io()
	x.identity = func(string) string { return "" }
	if ok, err := ReserveRelayAttention("» gtmux·agent-relay │ wait for HQ", func() (bool, error) { return true, nil }); !ok || err != nil {
		t.Fatal(ok, err)
	}
	f.nano += int64(time.Second)
	drain(x, "%isolated")
	if got := readQueue(); len(got) != 1 || !isRelayAttention(got[0].Name()) || f.pastes != 0 || f.enters != 0 {
		t.Fatal("unavailable target changed Relay or attempted key input", got, f)
	}
	if _, err := readIssue(issueTargetUnavailable); err != nil {
		t.Fatal("missing original unavailable-target marker", err)
	}
}

func seedRotationBriefRecovery(t *testing.T, home, outcome string) nativeIntent {
	t.Helper()
	bind, request, f, _ := rotationBriefFixture(t)
	t.Setenv("HOME", home)
	writeBriefRotation(t, request)
	interrupted := errors.New("synthetic interruption after recording the intent")
	nativeWakeIO = func(string, ...string) dispatch.IO {
		return dispatch.IO{
			CaptureColor: func() string { return f.capture(bind.Pane) },
			Sleep:        func() {},
			SubmitNative: func(_ dispatch.Opts, _ string, hold func() string) dispatch.Result {
				if why := hold(); why != "" {
					t.Fatal(why)
				}
				if outcome == "uncertain" {
					return dispatch.Result{Submission: dispatch.SubmissionUnknown}
				}
				panic(interrupted)
			},
		}
	}
	queueBoundBrief(t, bind)
	func() {
		defer func() {
			got := recover()
			if (outcome == "uncertain" && got != nil) || (outcome != "uncertain" && got != interrupted) {
				t.Fatalf("unexpected interruption at prepared intent: %v", got)
			}
		}()
		Drain(bind.Pane)
	}()
	files, err := os.ReadDir(nativeIntentDir())
	if err != nil || len(files) != 1 {
		t.Fatal("production drain did not write one original intent", files, err)
	}
	b, err := os.ReadFile(filepath.Join(nativeIntentDir(), files[0].Name()))
	var original nativeIntent
	if err != nil || json.Unmarshal(b, &original) != nil || len(original.Entries) != 1 {
		t.Fatal("invalid original intent", err)
	}
	if outcome == "accepted" {
		// Persist the real journal record at the accepted write, before cleanup.
		if original.Outcome != "prepared" {
			t.Fatal("missing prepared journal boundary", original.Outcome)
		}
		original.Outcome = outcome
		original.SubmittedAt = f.io().nowNano()
		original.WakeReceipt = &dispatch.WakeReceipt{
			Request: "original-receiver", Session: bind.Session, Home: bind.Home, Turn: "original-turn",
		}
		if err := writeNativeIntent(original); err != nil {
			t.Fatal(err)
		}
	}
	if original.Outcome != outcome || f.pastes != 0 || f.enters != 0 {
		t.Fatal("incorrect interruption fixture", original.Outcome, f)
	}
	return original
}

func verifyRotationBriefRecovery(t *testing.T, original nativeIntent) {
	t.Helper()
	f := &fake{base: time.Now().UnixNano()}
	x := f.io()
	const tick = "» gtmux·tick │ workspace reconciliation"
	const relay = "» gtmux·agent-relay │ protected relay"
	if ok, err := ReserveRelayAttention(relay, func() (bool, error) { return true, nil }); !ok || err != nil {
		t.Fatal(ok, err)
	}
	f.nano += int64(time.Second)
	enqueueKeyed(x, "workspace-reconciliation", tick, 0)
	calls := 0
	x.nativeSubmit = func(_ string, text string, _ string) (dispatch.Result, bool) {
		calls++
		if strings.Contains(text, "STARTUP BRIEFING") || !strings.Contains(text, tick) || !strings.Contains(text, relay) {
			t.Errorf("unexpected recovered delivery: %q", text)
		}
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	drain(x, original.Pane)
	if calls != 0 || f.pastes != 0 || f.enters != 0 {
		t.Fatal("restart crossed the current handoff reservation", calls)
	}
	f.nano += int64(3 * time.Second) // the original fixture's handoff deadline has passed
	drain(x, original.Pane)
	drain(x, original.Pane)
	if calls != 1 || f.pastes != 0 || f.enters != 0 {
		t.Errorf("recovery interrupted ordinary wakes or attempted keys: native=%d paste=%d enter=%d", calls, f.pastes, f.enters)
	}
	if original.Outcome == "accepted" {
		if _, err := os.Stat(nativeIntentPath(original)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("the accepted intent should settle once its receipt is durable", err)
		}
		saved, err := readAcceptedWake(filepath.Join(acceptedWakeDir(), filepath.Base(nativeIntentPath(original))))
		if err != nil || saved.Pane != original.Pane || saved.Batch != original.ID || saved.Payload != original.Payload ||
			saved.Since != original.SubmittedAt || saved.Receipt != *original.WakeReceipt {
			t.Fatal("accepted recovery lost the original receipt", err, saved)
		}
		return
	}
	b, err := os.ReadFile(nativeIntentPath(original))
	var retained nativeIntent
	if err != nil || json.Unmarshal(b, &retained) != nil || retained.Outcome != "uncertain" ||
		retained.ID != original.ID || retained.Payload != original.Payload || !reflect.DeepEqual(retained.Entries, original.Entries) {
		t.Fatal("recovery changed the original possible submission", err, retained)
	}
	path := filepath.Join(queueDir(), retained.Entries[0].Name)
	parked, err := nativeFingerprint(finalClaimPath(retained, path))
	if err != nil || parked.SHA256 != retained.Entries[0].SHA256 || parked.FileID != retained.Entries[0].FileID {
		t.Fatal("recovery changed the original claim", err, parked)
	}
}
