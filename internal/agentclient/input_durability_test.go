package agentclient

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var injectedInputFault = errors.New("injected input persistence fault")

func prepareDurabilityFixture(t *testing.T) (Request, Attachment, string) {
	t.Helper()
	return prepareFixtureWithInputID(t, "fixed-input")
}

func prepareFixtureWithInputID(t *testing.T, id string) (Request, Attachment, string) {
	t.Helper()
	_, a, _ := attachmentFixture(t)
	r, err := Prepare(a, "durable fixture", id, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	indexes, err := filepath.Glob(filepath.Join(r.store.path, "*.index.json"))
	if err != nil || len(indexes) != 1 {
		t.Fatalf("want one index: %v, %v", indexes, err)
	}
	return r, a, filepath.Base(indexes[0])
}

func assertInputName(t *testing.T, r Request, suffix string, present bool) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(r.store.path, r.ID+suffix))
	if present && err != nil || !present && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s present=%t: %v", suffix, present, err)
	}
}

func TestSPL011ClaimFlushPrecedesInputAuthority(t *testing.T) {
	for _, stage := range []string{"file", "directory", "success"} {
		t.Run(stage, func(t *testing.T) {
			r, a, _ := prepareDurabilityFixture(t)
			ops := syncInput
			var trace []string
			ops.syncFile = func(f *os.File) error {
				trace = append(trace, "file")
				if stage == "file" {
					return injectedInputFault
				}
				return syncInput.syncFile(f)
			}
			ops.syncDir = func(f *os.File) error {
				trace = append(trace, "directory")
				if stage == "directory" {
					return injectedInputFault
				}
				return syncInput.syncDir(f)
			}
			ops.unlink = func(fd int, name string, flags int) error {
				trace = append(trace, "unlink "+name)
				return syncInput.unlink(fd, name, flags)
			}
			err := claimNative(r, ops)
			if stage == "success" {
				if err != nil || strings.Join(trace, ",") != "file,file,directory,unlink "+r.ID+".pending.json" {
					t.Fatalf("wrong claim barrier order: %v %v", trace, err)
				}
				assertInputName(t, r, ".pending.json", false)
				return
			}
			if !errors.Is(err, injectedInputFault) || len(trace) == 0 || strings.Contains(strings.Join(trace, ","), "unlink") {
				t.Fatalf("failed claim granted authority: %v %v", trace, err)
			}
			assertInputName(t, r, ".claimed.json", true)
			assertInputName(t, r, ".pending.json", true)
			if _, err := Prepare(a, r.Text, "fixed-input", true, time.Now()); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("failed claim could replay: %v", err)
			}
		})
	}
}

func TestSPL011ReceiptFlushPrecedesAckAndClaimCleanup(t *testing.T) {
	for _, stage := range []string{"file", "link", "directory", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			r, a, index := prepareDurabilityFixture(t)
			if err := ClaimNative(r); err != nil {
				t.Fatal(err)
			}
			ops := syncInput
			var trace []string
			ops.syncFile = func(f *os.File) error {
				trace = append(trace, "file")
				if stage == "file" {
					return injectedInputFault
				}
				return syncInput.syncFile(f)
			}
			ops.link = func(src int, tmp string, dst int, name string, flags int) error {
				trace = append(trace, "link")
				if stage == "link" {
					return injectedInputFault
				}
				return syncInput.link(src, tmp, dst, name, flags)
			}
			ops.syncDir = func(f *os.File) error {
				trace = append(trace, "directory")
				if stage == "directory" {
					return injectedInputFault
				}
				return syncInput.syncDir(f)
			}
			ops.unlink = func(fd int, name string, flags int) error {
				if strings.HasSuffix(name, ".claimed.json") {
					trace = append(trace, "claim cleanup")
					if stage == "cleanup" {
						return injectedInputFault
					}
				}
				return syncInput.unlink(fd, name, flags)
			}
			err := finishNativeDelivery(r, "accepted", "", "", ops)
			if stage == "cleanup" {
				if err != nil || strings.Join(trace, ",") != "file,link,directory,claim cleanup" {
					t.Fatalf("committed receipt changed on cleanup error: %v %v", trace, err)
				}
			} else if !errors.Is(err, injectedInputFault) || strings.Contains(strings.Join(trace, ","), "claim cleanup") {
				t.Fatalf("cleanup outran receipt commitment: %v %v", trace, err)
			}
			assertInputName(t, r, ".claimed.json", true)
			visible := stage == "directory" || stage == "cleanup"
			assertInputName(t, r, ".receipt.json", visible)
			if _, ok := readCommittedInputReceipt(r, ops); ok != (stage == "cleanup") {
				t.Fatalf("visible is not necessarily committed: stage=%s accepted=%t", stage, ok)
			}
			if stage == "directory" {
				q, err := openInputQueue(r.store, ops)
				if err != nil {
					t.Fatal(err)
				}
				defer q.dir.Close()
				if removed, err := q.reconcileInput(index, time.Now()); removed || err != nil {
					t.Fatalf("reconciliation trusted a failed receipt: %v, %v", removed, err)
				}
				assertInputName(t, r, ".claimed.json", true)
			}
			if visible {
				name := filepath.Join(r.store.path, r.ID+".receipt.json")
				before, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := FinishNative(r, "accepted", ""); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(name)
				if err != nil || string(after) != string(before) {
					t.Fatal("adoption overwrote the existing receipt", err)
				}
				assertInputName(t, r, ".claimed.json", false)
				a2, err := Prepare(a, r.Text, "fixed-input", true, time.Now())
				if err != nil || a2.ID != r.ID {
					t.Fatalf("durable acceptance lost identity: %+v %v", a2, err)
				}
			}
		})
	}
}

func TestSPL011CompetingAndUnknownReceiptsPreserveClaims(t *testing.T) {
	t.Run("no replacing publication", func(t *testing.T) {
		r, _, _ := prepareDurabilityFixture(t)
		if err := ClaimNative(r); err != nil {
			t.Fatal(err)
		}
		ops := syncInput
		ops.link = func(src int, tmp string, dst int, name string, flags int) error {
			putJSON(t, filepath.Join(r.store.path, name), Receipt{
				Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID,
				SessionID: r.SessionID, PID: r.PID, State: "uncertain", Error: "incumbent",
			})
			return syncInput.link(src, tmp, dst, name, flags)
		}
		if err := finishNativeDelivery(r, "accepted", "", "", ops); !errors.Is(err, os.ErrExist) {
			t.Fatalf("competing record was not protected: %v", err)
		}
		if got, ok := ReadReceipt(r); !ok || got.State != "uncertain" || got.Error != "incumbent" {
			t.Fatalf("competing outcome changed: %+v %t", got, ok)
		}
		assertInputName(t, r, ".claimed.json", true)
	})

	t.Run("uncertain outcome does not age out", func(t *testing.T) {
		r, _, index := prepareDurabilityFixture(t)
		if err := ClaimNative(r); err != nil {
			t.Fatal(err)
		}
		if err := FinishNative(r, "uncertain", "no result"); err != nil {
			t.Fatal(err)
		}
		q, err := openInputQueue(r.store, syncInput)
		if err != nil {
			t.Fatal(err)
		}
		defer q.dir.Close()
		before := gateSnapshot(t, r.store.path)
		if removed, err := q.reconcileInput(index, time.Now().Add(48*time.Hour)); removed || err != nil {
			t.Fatalf("uncertain outcome reclaimed: %t %v", removed, err)
		}
		gateEqual(t, before, gateSnapshot(t, r.store.path))
	})
}

func TestSPL012RetirementKeepsReceiptUntilPayloadRemovalIsDurable(t *testing.T) {
	r, _, index := prepareFixtureWithInputID(t, "")
	// Old DSH could leave an empty claim and a receipt without removing a
	// matching pending file, so retirement must protect both payload paths.
	if err := os.WriteFile(filepath.Join(r.store.path, r.ID+".claimed.json"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	putJSON(t, filepath.Join(r.store.path, r.ID+".receipt.json"), Receipt{
		Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID,
		SessionID: r.SessionID, PID: r.PID, State: "accepted", CreatedAt: r.CreatedAt,
	})
	var calls int
	ops := syncInput
	ops.syncDir = func(f *os.File) error {
		calls++
		if calls == 2 {
			return injectedInputFault
		}
		return syncInput.syncDir(f)
	}
	q, err := openInputQueue(r.store, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	if removed, err := q.reconcileInput(index, time.Now().Add(48*time.Hour)); removed || !errors.Is(err, injectedInputFault) {
		t.Fatalf("did not stop before retiring receipt: %t %v", removed, err)
	}
	assertInputName(t, r, ".receipt.json", true)
	if q.missing(index) || calls != 2 {
		t.Fatalf("lost retirement evidence: syncs=%d", calls)
	}
}

func TestSPL017IndexDeletionFailureKeepsReceipt(t *testing.T) {
	r, _, index := prepareFixtureWithInputID(t, "")
	if err := ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	if err := FinishNative(r, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	ops := syncInput
	ops.unlink = func(fd int, name string, flags int) error {
		if name == index {
			return injectedInputFault
		}
		return syncInput.unlink(fd, name, flags)
	}
	q, err := openInputQueue(r.store, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	if removed, err := q.reconcileInput(index, time.Now().Add(48*time.Hour)); removed || !errors.Is(err, injectedInputFault) {
		t.Fatalf("unexpected retirement: %t %v", removed, err)
	}
	assertInputName(t, r, ".receipt.json", true)
	if q.missing(index) {
		t.Fatal("index unexpectedly disappeared")
	}
}
