package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSPL006DamagedRecordsRetainTheirReplayProtection(t *testing.T) {
	for _, part := range []string{"index", "pending", "claimed", "receipt"} {
		t.Run(part, func(t *testing.T) {
			_, a, inspect := attachmentFixture(t)
			const body = "合成输入 😀 and `\\u1234`"
			r, err := Prepare(a, body, "stable-fixture", false, inspect.Now)
			if err != nil {
				t.Fatal(err)
			}
			dir := InputDir()
			indexes, err := filepath.Glob(filepath.Join(dir, "*.index.json"))
			if err != nil || len(indexes) != 1 {
				t.Fatalf("missing fixture index: %v %v", indexes, err)
			}
			path := filepath.Join(dir, r.ID+"."+part+".json")
			if part == "index" {
				path = indexes[0]
			}
			if part == "claimed" || part == "receipt" {
				if err := ClaimNative(r); err != nil {
					t.Fatal(err)
				}
				putJSON(t, filepath.Join(dir, r.ID+".receipt.json"), Receipt{
					Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, PID: r.PID, HomeID: r.HomeID,
					SessionID: r.SessionID, State: "accepted", CreatedAt: r.CreatedAt,
				})
				if receipt, ok := ReadReceipt(r); !ok || receipt.State != "accepted" {
					t.Fatalf("valid v1 receipt was refused: %+v", receipt)
				}
			}
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			damaged := append([]byte(`{"reque\u0073t_id":"`+r.ID+`",`), original[1:]...)
			if err := os.WriteFile(path, damaged, 0600); err != nil {
				t.Fatal(err)
			}
			before := gateSnapshot(t, dir)
			if part == "pending" {
				if err := ClaimNative(r); err == nil {
					t.Fatal("damaged pending became a claim")
				}
			}
			if part == "claimed" {
				queue, err := openInputQueue(r.store, syncInput)
				if err != nil {
					t.Fatal(err)
				}
				defer queue.dir.Close()
				if removed, err := queue.reconcileInput(filepath.Base(indexes[0]), inspect.Now.Add(48*time.Hour)); err != nil || removed {
					t.Fatalf("damaged claim was eligible for cleanup: %v, %v", removed, err)
				}
			} else if _, err := Prepare(a, body, "stable-fixture", true, inspect.Now.Add(48*time.Hour)); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("damaged %s did not retain replay protection: %v", part, err)
			}
			if part == "receipt" {
				if _, ok := ReadReceipt(r); ok {
					t.Fatal("damaged receipt recognized as accepted")
				}
			}
			gateEqual(t, before, gateSnapshot(t, dir))
		})
	}

	// Literal replacement characters and paired escapes remain valid JSON.
	valid := `{"a":"\ud83d\ude00","b":"�","c":"\\ud800"}`
	if err := checkInputJSON([]byte(valid)); err != nil || !json.Valid([]byte(valid)) {
		t.Fatalf("valid Unicode was refused: %v", err)
	}
	invalid := `{"a":"\ud800","b":"` + strings.Repeat("a", 2) + `"}`
	if err := checkInputJSON([]byte(invalid)); err == nil {
		t.Fatal("isolated UTF-16 surrogate was accepted")
	}
}
