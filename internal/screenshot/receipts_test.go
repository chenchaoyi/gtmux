package screenshot

import (
	"encoding/json"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASC005ReceiptsNeverReplayUncertainOrChangedRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := "synthetic-request-0001"
	if _, e := ReadReceipt(id); e == nil {
		t.Fatal("missing status fabricated")
	}
	if _, e := os.Stat(filepath.Join(state.Dir(), "capture-requests")); !os.IsNotExist(e) {
		t.Fatal("status created runtime state")
	}
	r, fresh, e := Claim(id, "target", strings.Repeat("a", 64), "private note")
	if e != nil || !fresh || r.State != "unknown" || r.Confirmed {
		t.Fatal(r, fresh, e)
	}
	again, fresh, e := Claim(id, "target", r.ImageSHA, "private note")
	if e != nil || fresh || again.PayloadSHA != r.PayloadSHA {
		t.Fatal(again, fresh, e)
	}
	if _, _, e = Claim(id, "other", r.ImageSHA, "private note"); e == nil {
		t.Fatal("payload conflict accepted")
	}
	for _, s := range []string{"queued", "blocked", "unknown", "landed", "active_turn_accepted"} {
		confirm := s == "landed" || s == "active_turn_accepted"
		r, e = Finish(r, s, "synthetic", confirm)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadReceipt(id)
		if e != nil || got.State != s || got.Confirmed != confirm {
			t.Fatal(got, e)
		}
	}
	if _, e = Finish(r, "queued", "", true); e == nil {
		t.Fatal("queue claimed confirmed")
	}
	dir, _ := PrivateDir()
	raw, _ := os.ReadFile(filepath.Join(dir, id+".json"))
	if strings.Contains(string(raw), "private note") || strings.Contains(string(raw), `"target"`) {
		t.Fatal("receipt stored content")
	}
	_ = os.Chmod(filepath.Join(dir, id+".json"), 0644)
	if _, e = ReadReceipt(id); e == nil {
		t.Fatal("public receipt read")
	}
}
func TestASC005ReceiptStoreRefusesCorruptionAndCapacity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, e := PrivateDir()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 1000; i++ {
		p := filepath.Join(dir, fmt.Sprintf("synthetic-%04d", i))
		if e = os.WriteFile(p, []byte("x"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, _, e = Claim("synthetic-request-0002", "t", strings.Repeat("a", 64), ""); AsError(e).Code != "receipt-budget-full" {
		t.Fatal(e)
	}
	var v Receipt
	if Decode([]byte(`{"unknown":true}`), &v) == nil {
		t.Fatal("unknown fields accepted")
	}
	b, _ := json.Marshal(v)
	if Decode(append(b, []byte(` {}`)...), &v) == nil {
		t.Fatal("trailing value accepted")
	}
}
