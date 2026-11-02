package hqworkspace

import (
	"encoding/json"
	"testing"
)

func TestRestoreValidationIsPureAndUsesCurrentSchema(t *testing.T) {
	// Resolving a host home here would panic under the test state guard.
	t.Setenv("HOME", "/not-a-disposable-home")
	d := Empty()
	d.Messages["m"] = Message{ID: "m", Delivery: "pending"}
	b, _ := json.Marshal(d)
	if e := ValidateRestoreRecords(b, 100); e != nil {
		t.Fatal(e)
	}
	var original Database
	_ = json.Unmarshal(b, &original)
	if original.Messages["m"].Delivery != "pending" {
		t.Fatal("validation changed source")
	}
	if e := ValidateRestoreRecords([]byte(`{"schema":"unknown"}`), 100); e == nil {
		t.Fatal("invalid schema accepted")
	}
}
