package agentclient

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSPL020StableReceiptsAllowLongRunningAdmission(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	now := time.Now()
	accepted := settleRetirementFixture(t, target, "accepted", "accepted", now)
	refused := settleRetirementFixture(t, target, "was-cancelled", "refused", now)
	unknown := settleRetirementFixture(t, target, "unknown", "uncertain", now)
	pending, err := Prepare(target, "in flight", "pending", false, now)
	if err != nil {
		t.Fatal(err)
	}
	view, err := Prepare(target, accepted.Text, "accepted", false, now.Add(48*time.Hour))
	if err != nil || view.ID != accepted.ID {
		t.Fatalf("retention lost original ID after 48h: %+v %v", view, err)
	}
	if receipt, ok := ReadReceipt(view); !ok || receipt.State != "accepted" {
		t.Fatalf("retention lost original receipt: %+v", receipt)
	}

	dir := InputDir()
	// Seed more complete operations than the original steady-state DSH
	// capacity, with a mixture of compact Go and legacy empty DSH claims.
	// Real admissions and settlement run below over multiple later windows.
	for n := range 1450 {
		seed := fmt.Sprintf("settled-%d", n)
		sum := sha256.Sum256([]byte(seed))
		id := fmt.Sprintf("%x", sum[:16])
		r := accepted.persisted()
		r.ID, r.Text, r.IndexKey = id, "", strings.TrimSuffix(deliveryIndex(target.Pane, seed), ".index.json")
		putJSON(t, filepath.Join(dir, r.IndexKey+".index.json"), r)
		putJSON(t, filepath.Join(dir, r.ID+".receipt.json"), Receipt{
			Version: r.Version, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID,
			SessionID: r.SessionID, PID: r.PID, State: "accepted", CreatedAt: now.UnixMilli(),
		})
		if n < 1150 {
			if err := os.WriteFile(filepath.Join(dir, r.ID+".claimed.json"), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for n := 0; ; n++ {
		err := admitInput(dir)
		if errors.Is(err, ErrInputFull) {
			break
		}
		if err != nil || n > 60 {
			t.Fatalf("fixture did not approach capacity: %d %v", n, err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("foreign-%d", n)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ClaimNative(pending); err != nil {
		t.Fatal("could not claim the reserved in-flight operation", err)
	}
	if err := FinishNative(pending, "accepted", ""); err != nil {
		t.Fatal("could not settle the reserved in-flight operation", err)
	}
	for n := 1; n <= 6; n++ {
		date := now.Add(time.Duration(n) * 96 * time.Hour)
		name := fmt.Sprintf("new-window-%d", n)
		settleRetirementFixture(t, target, name, "accepted", date)
	}
	// Pruning is intentionally limited to a small batch per admission.
	if _, err := os.Stat(filepath.Join(dir, accepted.ID+".receipt.json")); err == nil {
		advanced := now.Add(30 * 24 * time.Hour)
		plan, err := StageInput(target, "advance retirement", "advance-retirement", false, advanced)
		if err != nil {
			t.Fatal(err)
		}
		q, err := openInputQueue(accepted.store, syncInput)
		if err != nil {
			t.Fatal(err)
		}
		defer q.dir.Close()
		removed, err := q.reconcileInput(accepted.IndexKey+".index.json", advanced)
		if err != nil || !removed {
			t.Fatalf("retirement failed: %v", err)
		}
		if _, err := plan.Publish(advanced); err != nil {
			t.Fatal(err)
		}
	}
	late := now.Add(31 * 24 * time.Hour)
	for _, old := range []struct {
		id  string
		req Request
	}{{"accepted", accepted}, {"unknown", unknown}} {
		if _, err := Prepare(target, old.req.Text, old.id, true, late); !errors.Is(err, ErrInputUncertain) {
			t.Fatalf("%s became eligible for re-submission: %v", old.id, err)
		}
		if err := TerminalFallbackError(target.Pane, old.id); err == nil {
			t.Fatalf("%s could fall back to raw input", old.id)
		}
	}
	assertInputName(t, accepted, ".receipt.json", false)
	assertInputName(t, unknown, ".claimed.json", true)
	assertInputName(t, unknown, ".receipt.json", true)
	// Proven zero mutation may expire. A new attempt is then distinct.
	q, err := openInputQueue(refused.store, syncInput)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	if _, err := q.reconcileInput(refused.IndexKey+".index.json", late); err != nil {
		t.Fatal(err)
	}
	retry, err := Prepare(target, refused.Text, "was-cancelled", false, late)
	if err != nil || retry.ID == refused.ID {
		t.Fatalf("known no-submit did not retire: %v", err)
	}
}

func TestSPL021InterruptedRetirementReconcilesExactTail(t *testing.T) {
	for _, step := range []string{"payload barrier", "index unlink", "receipt unlink", "receipt barrier"} {
		t.Run(step, func(t *testing.T) {
			_, target, _ := attachmentFixture(t)
			now := time.Now()
			age := now.Add(96 * time.Hour)
			r := settleRetirementFixture(t, target, "stable", "accepted", now)
			orphan := strings.Repeat("9", 32) + ".receipt.json"
			if err := os.WriteFile(filepath.Join(InputDir(), orphan), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			claimed := r.ID + ".claimed.json"
			if err := os.WriteFile(filepath.Join(InputDir(), claimed), nil, 0600); err != nil {
				t.Fatal(err)
			}
			index, receipt := r.IndexKey+".index.json", r.ID+".receipt.json"
			ops := syncInput
			removedPayload, removedReceipt := false, false
			ops.unlink = func(fd int, name string, flags int) error {
				if (step == "index unlink" && name == index) || (step == "receipt unlink" && name == receipt) {
					return injectedInputFault
				}
				err := syncInput.unlink(fd, name, flags)
				removedPayload = removedPayload || name == claimed && err == nil
				removedReceipt = removedReceipt || name == receipt && err == nil
				return err
			}
			ops.syncDir = func(f *os.File) error {
				if (step == "receipt barrier" && removedReceipt) || (step == "payload barrier" && removedPayload) {
					return injectedInputFault
				}
				return syncInput.syncDir(f)
			}
			q, err := openInputQueue(r.store, ops)
			if err != nil {
				t.Fatal(err)
			}
			removed, err := q.reconcileInput(index, age)
			q.dir.Close()
			if removed || !errors.Is(err, injectedInputFault) {
				t.Fatalf("fault did not interrupt retirement: %v %v", removed, err)
			}
			if step == "receipt unlink" {
				assertInputName(t, r, ".receipt.json", true)
				if _, err := os.Stat(filepath.Join(InputDir(), index)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("fixture did not create an unindexed receipt: %v", err)
				}
			}
			if err := TerminalFallbackError(target.Pane, "stable"); err == nil {
				t.Fatal("old input lost its fallback protection")
			}
			if _, err := Prepare(target, "new", "after-failure", false, age); err != nil {
				t.Fatal("next admission could not reconcile the exact tail", err)
			}
			assertInputName(t, r, ".receipt.json", false)
			if _, err := Prepare(target, r.Text, "stable", false, age); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("old input got new authority after recovery: %v", err)
			}
			if _, err := os.Stat(filepath.Join(InputDir(), orphan)); err != nil {
				t.Fatalf("unattributed orphan was removed: %v", err)
			}
		})
	}
}

func TestSPL021FullArchiveShardRetainsAcceptance(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	now := time.Now()
	r := settleRetirementFixture(t, target, "full-archive-shard", "accepted", now)
	path := filepath.Join(InputDir(), retiredInputDir)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	shard, _, ok := retiredKey(r.IndexKey + ".index.json")
	if !ok {
		t.Fatal("invalid fixture key")
	}
	if err := os.WriteFile(filepath.Join(path, shard), make([]byte, retiredShardLimit), 0600); err != nil {
		t.Fatal(err)
	}
	late := now.Add(96 * time.Hour)
	if _, err := Prepare(target, "fresh", "fresh-id", false, late); err != nil {
		t.Fatalf("a different ID could not use available slots: %v", err)
	}
	if got, err := Prepare(target, r.Text, "full-archive-shard", false, late); err != nil || got.ID != r.ID {
		t.Fatalf("full archive shard discarded accepted input: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(path, retirementTail)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("full shard left behind a blocking checkpoint: %v", err)
	}
}

func TestSPL022CancelDuringParentSyncDoesNotPublish(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	now := time.Now()
	plan, err := StageInput(target, "fixture", "cancel-during-flush", false, now)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ops := syncInput
	ops.syncParent = func(f *os.File) error { cancel(); return syncInput.syncParent(f) }
	if _, err = plan.publishContext(ctx, now, ops); !errors.Is(err, context.Canceled) {
		t.Fatalf("publication did not observe cancellation: %v", err)
	}
	names, err := inputNames(InputDir())
	if err != nil || len(names) != 0 {
		t.Fatalf("cancellation consumed durable slots: %v %v", names, err)
	}
}

func settleRetirementFixture(t *testing.T, target Attachment, id, state string, now time.Time) Request {
	t.Helper()
	r, err := Prepare(target, "fixture", id, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	if err := FinishNative(r, state, ""); err != nil {
		t.Fatal(err)
	}
	return r
}
