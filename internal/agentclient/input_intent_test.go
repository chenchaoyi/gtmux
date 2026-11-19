package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL016RepeatedRefusalsLeaveNoPersistentSlot(t *testing.T) {
	_, a, _ := attachmentFixture(t)
	for _, guard := range []string{"draft", "waiting", "held-after-paste", "duplicate"} {
		for range 3 {
			frame, pastes := "⌸ ❯ ", 0
			opts := dispatch.Opts{Pane: a.Pane, HasComposer: true, DeliveryID: guard}
			io := dispatch.IO{
				Capture: func() string { return frame },
				Now:     func() int64 { return 100 }, Sleep: func() {},
				Paste: func(s string) error { pastes++; frame = "⌸ ❯ " + s; return nil },
				Enter: func() error { t.Fatal("refusal reached Enter"); return nil },
			}
			if guard == "draft" {
				frame = "⌸ ❯ existing draft"
			}
			io.Hold = func() string {
				if guard == "waiting" || guard == "held-after-paste" && pastes != 0 {
					return "fixture hold"
				}
				return ""
			}
			if guard == "duplicate" {
				io.RecentSend = func(string) (string, int64) { return dispatch.PayloadHash(a.Pane, "fixture"), 100 }
				opts.ResendWindow = 20
			}
			got := dispatch.Deliver(WrapIO(a.Pane, io, func(string) (Attachment, bool) { return a, true }), opts, "fixture")
			if got.Submitted() || got.Attempts != 0 || (guard == "held-after-paste" && !strings.HasPrefix(got.Evidence, dispatch.EvidenceHeldBeforeEnter)) {
				t.Fatalf("guard %s: %+v", guard, got)
			}
		}
	}
	if _, err := os.Lstat(state.Dir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused sends created state: %v", err)
	}
}

func TestSPL016PublisherUsesStagedIDAndNeverReplacesUnresolved(t *testing.T) {
	_, a, _ := attachmentFixture(t)
	now := time.Now()
	first, err := StageInput(a, "same", "race-id", false, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := StageInput(a, "same", "race-id", false, now)
	if err != nil {
		t.Fatal(err)
	}
	r, err := first.Publish(now)
	if err != nil || r.ID != first.Request.ID {
		t.Fatalf("published a different command: %+v %v", r, err)
	}
	if _, err := second.Publish(now); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("second writer replaced evidence: %v", err)
	}
	kept, err := StageInput(a, "same", "race-id", true, now.Add(72*time.Hour))
	if err != nil || kept.Request.ID != r.ID {
		t.Fatalf("old pending split at retry: %+v %v", kept, err)
	}
	if _, err := Prepare(a, "old non-durable", "", false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := StageInput(a, "old non-durable", "", true, now.Add(time.Hour)); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("aged unresolved non-durable input was replaced: %v", err)
	}
}

func TestSPL017PublicationFaultsNeverExposePendingBeforeIndex(t *testing.T) {
	for _, kind := range []string{"fresh parent", "existing index file", "existing index link", "existing queue barrier"} {
		t.Run(kind, func(t *testing.T) {
			_, a, _ := attachmentFixture(t)
			if kind != "fresh parent" {
				if err := os.MkdirAll(InputDir(), 0700); err != nil {
					t.Fatal(err)
				}
			}
			now := time.Now()
			plan, err := StageInput(a, "fixture", "durable", false, now)
			if err != nil {
				t.Fatal(err)
			}
			ops := syncInput
			switch kind {
			case "fresh parent":
				ops.syncParent = func(*os.File) error { return errInjectedInputFault }
			case "existing index file":
				ops.syncFile = func(*os.File) error { return errInjectedInputFault }
			case "existing index link":
				ops.link = func(int, string, int, string, int) error { return errInjectedInputFault }
			default:
				ops.syncDir = func(*os.File) error { return errInjectedInputFault }
			}
			_, err = plan.publish(now, ops)
			if !errors.Is(err, errInjectedInputFault) {
				t.Fatalf("fault was not reached: %v", err)
			}
			dir := InputDir()
			if _, err := os.Lstat(filepath.Join(dir, plan.Request.ID+".pending.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("pending was visible before the index barrier: %v", err)
			}
			_, err = os.Lstat(filepath.Join(dir, plan.name))
			if kind == "existing queue barrier" && err != nil || kind != "existing queue barrier" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("indeterminate index not preserved or premature mapping: %v", err)
			}
		})
	}
}

func TestSPL017DurableIDAcrossProcessRestart(t *testing.T) {
	_, a, _ := attachmentFixture(t)
	now := time.Now()
	r, err := Prepare(a, "fixture", "stable-restart", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	if err := FinishNative(r, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestSPL017RestartChild$")
	cmd.Env = append(os.Environ(), "GSL17_TARGET="+string(payload), "GSL17_ID="+r.ID)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restart did not preserve idempotency: %v\n%s", err, out)
	}
}

func TestSPL017RestartChild(t *testing.T) {
	target := os.Getenv("GSL17_TARGET")
	if target == "" {
		return
	}
	var a Attachment
	if err := json.Unmarshal([]byte(target), &a); err != nil {
		t.Fatal(err)
	}
	got, err := Prepare(a, "fixture", "stable-restart", true, time.Now().Add(48*time.Hour))
	if err != nil || got.ID != os.Getenv("GSL17_ID") {
		t.Fatalf("restart changed ID: %+v %v", got, err)
	}
	if receipt, ok := ReadReceipt(got); !ok || receipt.State != "accepted" {
		t.Fatalf("restart lost receipt: %+v, %t", receipt, ok)
	}
}
