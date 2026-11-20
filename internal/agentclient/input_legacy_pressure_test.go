package agentclient

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyInputPressurePreservesPriorDeliveries(t *testing.T) {
	_, target, fixture := attachmentFixture(t)
	now := fixture.Now
	accepted := settleRetirementFixture(t, target, "legacy-accepted", "accepted", now)
	seed, err := Prepare(target, "legacy payload", "legacy-pending", false, now)
	if err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(InputDir(), seed.ID+".pending.json")
	pending, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatal(err)
	}

	// The older writer counted only the two names it created. The current
	// writer must also reserve the claim and receipt names.
	for i := 1; i <= 1300; i++ {
		copy := seed
		copy.ID = fmt.Sprintf("%032x", i)
		copy.IndexKey = fmt.Sprintf("%032x", i+4096)
		putJSON(t, filepath.Join(InputDir(), copy.ID+".pending.json"), copy)
		copy.Text = ""
		putJSON(t, filepath.Join(InputDir(), copy.IndexKey+".index.json"), copy)
	}
	settleRetirementFixture(t, target, "new-after-upgrade", "accepted", now)

	got, err := os.ReadFile(pendingPath)
	if err != nil || !bytes.Equal(pending, got) {
		t.Fatalf("admission changed an existing pending input: %v", err)
	}
	retry, err := Prepare(target, accepted.Text, "legacy-accepted", false, now)
	if err != nil || retry.ID != accepted.ID {
		t.Fatalf("accepted stable ID lost its original receipt: %v", err)
	}
	if receipt, ok := ReadReceipt(retry); !ok || receipt.State != "accepted" {
		t.Fatal("prior acceptance is unavailable")
	}
}
