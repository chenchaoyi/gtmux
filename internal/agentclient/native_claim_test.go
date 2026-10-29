package agentclient

import (
	"strings"
	"testing"
	"time"
)

func TestDirectNativeClaimSurvivesMissingReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	a := Attachment{Version: 1, Pane: "%9", PID: 101, Instance: strings.Repeat("a", 32), HomeID: "synthetic-home", SessionID: "root-a", InputReady: true}
	r, err := Prepare(a, "synthetic input", "action-a", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	if err := ClaimNative(r); err == nil {
		t.Fatal("same native input claimed twice")
	}
	if _, err := Prepare(a, "synthetic input", "action-a", false, now.Add(time.Hour)); err == nil {
		t.Fatal("missing receipt authorized replay")
	}
	if err := FinishNative(r, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	again, err := Prepare(a, "synthetic input", "action-a", false, now.Add(time.Minute))
	if err != nil || again.ID != r.ID {
		t.Fatal("accepted request lost idempotency", err)
	}
}
