package agentclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestInputSpoolHealthAgreesWithAdmission(t *testing.T) {
	_, target, fixture := attachmentFixture(t)
	dir := InputDir()
	usage, err := InspectInputSpool()
	if err != nil || usage.Entries != 0 || usage.UsedSlots != 0 {
		t.Fatalf("empty spool: %+v, %v", usage, err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspection created the queue: %v", err)
	}
	request, err := Prepare(target, "unresolved request", "health-fixture", false, fixture.Now)
	if err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(dir, request.ID+".pending.json")
	original, err := os.ReadFile(pending)
	if err != nil {
		t.Fatal(err)
	}
	fill := func(from, to int) {
		t.Helper()
		for i := from; i < to; i++ {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("other-%d", i)), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	room := inputLimit - 2*inputReservation
	fill(0, room)
	usage, err = InspectInputSpool()
	if err != nil || usage.Entries != room+2 || usage.UsedSlots != inputLimit-inputReservation {
		t.Fatalf("one admission remaining: %+v, %v", usage, err)
	}
	if err := admitInput(dir); err != nil {
		t.Fatalf("one operation should fit: %v", err)
	}
	fill(room, room+1)
	usage, err = InspectInputSpool()
	if err != nil || usage.Entries >= usage.ScanLimit || usage.UsedSlots+usage.Reservation != usage.SlotLimit+1 {
		t.Fatalf("expected weighted saturation before the scan bound: %+v, %v", usage, err)
	}
	if err := admitInput(dir); !errors.Is(err, ErrInputFull) {
		t.Fatalf("expected admission to agree: %v", err)
	}
	fill(room+1, inputScanLimit-1)
	usage, err = InspectInputSpool()
	if !errors.Is(err, ErrInputFull) || usage.Entries != inputScanLimit+1 || usage.UsedSlots != 0 {
		t.Fatalf("over-limit scan must be reported as unmeasured: %+v, %v", usage, err)
	}
	if got, err := os.ReadFile(pending); err != nil || string(got) != string(original) {
		t.Fatalf("inspection modified pending input: %v", err)
	}
}
