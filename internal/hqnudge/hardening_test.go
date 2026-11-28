package hqnudge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCHH001_TerminalWakeArchivalCapsIntentsAndPins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{fg: "codex"}
	x := f.io()
	x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
		return dispatch.Result{
			State:      dispatch.StateFailed,
			Submission: dispatch.SubmissionUnknown,
		}, true
	}

	const totalUncertain = 215
	for i := 0; i < totalUncertain; i++ {
		enqueue(x, fmt.Sprintf("» gtmux·done │ %%1 turn %d", i))
		drain(x, "%0")
	}

	intents, err := os.ReadDir(nativeIntentDir())
	if err != nil {
		t.Fatalf("read nativeIntentDir: %v", err)
	}
	if len(intents) > terminalRetainedLimit {
		t.Fatalf("native intents = %d, want <= %d", len(intents), terminalRetainedLimit)
	}
	pins, err := os.ReadDir(nativePinDir())
	if err != nil {
		t.Fatalf("read nativePinDir: %v", err)
	}
	if len(pins) > terminalRetainedLimit {
		t.Fatalf("native pins = %d, want <= %d", len(pins), terminalRetainedLimit)
	}
	archiveBytes, err := os.ReadFile(terminalArchivePath())
	if err != nil || len(archiveBytes) == 0 {
		t.Fatalf("expected non-empty terminal-archive.jsonl, err=%v", err)
	}

	// Subsequent wake must still deliver cleanly instead of bricking at 200 intents.
	delivered := 0
	x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
		delivered++
		return dispatch.Result{
			State: dispatch.StateLanded,
		}, true
	}
	enqueue(x, "» gtmux·waiting │ %2 ready after archival")
	drain(x, "%0")
	if delivered != 1 {
		t.Fatalf("delivered = %d after 215 uncertain wakes, want 1", delivered)
	}
}

func TestCHH001_CollidingSlotReconcilesAndClearsClaimConflict(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	baseNow := time.Now().UnixNano()
	f := &fake{fg: "codex", base: baseNow}
	x := f.io()

	// Step 1: produce a legitimate parked .native-uncertain wake for a keyed slot.
	x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
		return dispatch.Result{
			State:      dispatch.StateFailed,
			Submission: dispatch.SubmissionUnknown,
		}, true
	}
	enqueueKeyed(x, "slot-collide", "» gtmux·unread │ older-uncertain", 0)
	drain(x, "%0")

	var uncertainPath string
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), nativeUncertainSuffix) {
			uncertainPath = filepath.Join(queueDir(), e.Name())
			break
		}
	}
	if uncertainPath == "" {
		t.Fatal("expected parked .native-uncertain file")
	}
	baseName := strings.TrimSuffix(filepath.Base(uncertainPath), nativeUncertainSuffix)
	sendingName := baseName + sendingSuffix
	sendingPath := filepath.Join(queueDir(), sendingName)

	// Step 2: plant a distinct aged .sending claim + prepared nativeIntent for the same slot.
	if err := state.WritePrivateFile(sendingPath, []byte("» gtmux·unread │ newer-claim\n")); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-3 * orphanClaimAge)
	for _, p := range []string{uncertainPath, sendingPath} {
		if err := os.Chtimes(p, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	validID := "#" + batchID([]string{sendingPath}, "» gtmux·unread │ newer-claim")
	intent, err := prepareNativeIntent("%0", validID, "» gtmux·unread │ newer-claim · "+validID, []string{sendingPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(nativeIntentPath(intent), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	recordIssue(issueClaimConflict, baseNow-int64(2*time.Second))

	// Step 3: drain recovers and reconciles the aged collision, archiving both digests.
	x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
		return dispatch.Result{
			State: dispatch.StateLanded,
		}, true
	}
	f.nano += int64(5 * time.Second)
	enqueue(x, "» gtmux·waiting │ %3 follow-up")
	drain(x, "%0")

	if _, err := os.Lstat(sendingPath); !os.IsNotExist(err) {
		t.Fatalf("colliding .sending remained on disk: %v", err)
	}
	archiveRaw, err := os.ReadFile(terminalArchivePath())
	if err != nil {
		t.Fatalf("read terminal archive: %v", err)
	}
	foundCollision := false
	for _, line := range bytes.Split(bytes.TrimSpace(archiveRaw), []byte{'\n'}) {
		var rec terminalArchiveRecord
		if json.Unmarshal(line, &rec) == nil && rec.Kind == "collision" && rec.SHA256 != "" && rec.CollisionSHA256 != "" {
			foundCollision = true
			break
		}
	}
	if !foundCollision {
		t.Fatalf("expected collision archive record with both digests, got:\n%s", string(archiveRaw))
	}

	// Next drain sees zero conflicts and confirmed delivery > issue.LastAt clears issueClaimConflict.
	f.nano += int64(5 * time.Second)
	enqueue(x, "» gtmux·waiting │ %4 after recovery")
	drain(x, "%0")
	noteConfirmedDelivery((baseNow + f.nano) / int64(time.Second))
	health := ReadHealth((baseNow + f.nano) / int64(time.Second))
	for _, iss := range health.Issues {
		if iss.Reason == issueClaimConflict {
			t.Fatalf("issueClaimConflict remained active after clean reconciliation: %+v", health)
		}
	}
}

func TestCHH001_DraftAndQuestionRefusalsHoldWithoutExhausting(t *testing.T) {
	for _, code := range []dispatch.InputFailureCode{dispatch.InputDraftPresent, dispatch.InputQuestion} {
		t.Run(string(code), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{fg: "codex", base: time.Now().UnixNano()}
			now := f.base
			x := f.io()
			x.nowNano = func() int64 { return now }

			attempts := 0
			holdUser := true
			x.nativeSubmit = func(pane, payload, id string) (dispatch.Result, bool) {
				attempts++
				if holdUser {
					return dispatch.Result{
						State:       dispatch.StateRefusedDraft,
						Submission:  dispatch.SubmissionNotSubmitted,
						FailureCode: code,
					}, true
				}
				return dispatch.Result{
					State: dispatch.StateLanded,
				}, true
			}

			enqueue(x, "» gtmux·waiting │ %5 hold while typing")
			for i := 0; i < 6; i++ {
				drain(x, "%0")
				now += int64(20 * time.Second)
			}
			if attempts != 6 {
				t.Fatalf("attempts = %d, want 6 retries while user holds composer", attempts)
			}
			for _, e := range readQueue() {
				if strings.HasSuffix(e.Name(), wakeRefusedSuffix) {
					t.Fatalf("user-input hold exhausted into .refused: %s", e.Name())
				}
			}

			holdUser = false
			drain(x, "%0")
			if attempts != 7 {
				t.Fatalf("expected delivery once composer cleared, attempts=%d", attempts)
			}
			if queuedCount(t) != 0 {
				t.Fatalf("queue still holds %d entries after delivery", queuedCount(t))
			}
		})
	}
}
