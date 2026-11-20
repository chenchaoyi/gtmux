package agentclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL001AdmissionReservesCompletion(t *testing.T) {
	_, a, c := attachmentFixture(t)
	first, err := Prepare(a, "first", "one", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < inputLimit-2*inputReservation; i++ {
		if err = os.WriteFile(filepath.Join(InputDir(), fmt.Sprintf("other-%d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	winners := make(chan Request, 2)
	for _, key := range []string{"two", "three"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for attempts := 0; attempts < 50; attempts++ {
				r, e := Prepare(a, key, key, false, c.Now)
				if e == nil {
					winners <- r
					return
				}
				if !strings.Contains(e.Error(), "lease") {
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	close(winners)
	if len(winners) != 1 {
		t.Fatalf("wanted one remaining reservation, got %d", len(winners))
	}
	for _, r := range []Request{first, <-winners} {
		if err := ClaimNative(r); err != nil {
			t.Fatal(err)
		}
		if err := FinishNative(r, "accepted", ""); err != nil {
			t.Fatal(err)
		}
	}
	retry, err := Prepare(a, "first", "one", false, c.Now)
	if err != nil || retry.ID != first.ID {
		t.Fatal("same-ID lookup lost", err)
	}
}

func TestSPL002SettlementAndUnconfirmedRecovery(t *testing.T) {
	_, a, c := attachmentFixture(t)
	r, err := Prepare(a, "fixture", "one", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	lock, err := state.TryLease("terminal-input")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- FinishNative(r, "accepted", "") }()
	select {
	case e := <-done:
		lock.Close()
		t.Fatalf("completion bypassed lease: %v", e)
	case <-time.After(35 * time.Millisecond):
	}
	lock.Close()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = FinishNative(r, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if err = FinishNative(r, "uncertain", "timeout"); err == nil {
		t.Fatal("contradictory outcome replaced a receipt")
	}
	if _, err = os.Stat(filepath.Join(InputDir(), r.ID+".claimed.json")); !os.IsNotExist(err) {
		t.Fatal("settled body retained", err)
	}
	pending, err := Prepare(a, "pending", "two", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = FinishNative(pending, "accepted", ""); err == nil {
		t.Fatal("unclaimed action marked accepted")
	}
	if err = ClaimNative(pending); err != nil {
		t.Fatal(err)
	}
	if err = FinishNative(pending, "uncertain", "timeout"); err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(a, "pending", "two", false, c.Now.Add(72*time.Hour)); err == nil {
		t.Fatal("uncertain receipt replayed")
	}
	idx, err := Prepare(a, "broken", "three", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(InputDir(), idx.ID+".receipt.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(a, "broken", "three", false, c.Now.Add(72*time.Hour)); err == nil {
		t.Fatal("invalid receipt treated as missing")
	}

	// A manual draft override does not replace an in-flight request or
	// authorize a replay of a claimed input with an unknown outcome.
	first, err := Prepare(a, "manual draft override", "", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	forced, err := Prepare(a, first.Text, "", true, c.Now)
	if err != nil || forced.ID != first.ID {
		t.Fatalf("force replaced a pending request: %v", err)
	}
	if err := ClaimNative(first); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(a, first.Text, "", true, c.Now); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("force bypassed an unresolved claim: %v", err)
	}

	// Neither an index nor a matching message body proves that the saved
	// pending record still names the same terminal attachment.
	addressed, err := Prepare(a, "pending address changed", "four", false, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	replaced := addressed
	replaced.BindingEpoch++
	putJSON(t, filepath.Join(InputDir(), addressed.ID+".pending.json"), replaced)
	if _, err := Prepare(a, addressed.Text, "four", false, c.Now); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("reused a mismatched pending request: %v", err)
	}
}
