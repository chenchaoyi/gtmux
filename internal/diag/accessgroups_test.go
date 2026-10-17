package diag

import (
	"testing"
)

func TestAccessFiltersAndGroupsUseFullIdentityAcrossPages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 3; i++ {
		For("serve").Access(AccessEntry{Actor: "phone:a", DeviceID: "full-a", Name: "Same name", Scope: "device", Operation: "send", Outcome: OK})
	}
	For("serve").Access(AccessEntry{Actor: "phone:b", DeviceID: "full-b", Name: "Same name", Scope: "device", Operation: "send", Outcome: Failed})
	For("serve").Access(AccessEntry{Actor: "phone:a", DeviceID: "full-a", Name: "Renamed", Scope: "device", Operation: "terminal", Outcome: OK})
	p, err := ReadAccess(AccessQuery{DeviceID: "full-a", Operation: "send", Outcome: OK})
	if err != nil || len(p.Entries) != 3 || len(p.Groups) != 1 || p.Groups[0].Count != 3 {
		t.Fatalf("filters not applied before paging: %+v %v", p, err)
	}
	onlyB, err := ReadAccess(AccessQuery{DeviceID: "full-b"})
	if err != nil || len(onlyB.Entries) != 1 || onlyB.Entries[0].DeviceID != "full-b" {
		t.Fatal("exact device filter lost", onlyB, err)
	}
	counts := map[string]int{}
	names := map[string]string{}
	cursor := ""
	for i := 0; i < 10; i++ {
		page, err := ReadAccess(AccessQuery{GroupBy: "device", Limit: 1, Before: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Entries) != 0 {
			t.Fatal("summary retained unbounded raw entries")
		}
		for _, g := range page.Groups {
			counts[g.Key] += g.Count
			if names[g.Key] == "" {
				names[g.Key] = g.Name
			}
		}
		cursor = page.Next
		if cursor == "" {
			break
		}
	}
	if counts["device:full-a"] != 4 || counts["device:full-b"] != 1 || names["device:full-a"] != "Renamed" {
		t.Fatalf("same-name devices or repeated page lost: %v %v", counts, names)
	}
	page, err := ReadAccess(AccessQuery{Outcome: Failed, GroupBy: "device"})
	if err != nil || len(page.Groups) != 1 || page.Groups[0].Failed != 1 || page.Groups[0].Key != "device:full-b" {
		t.Fatalf("outcome summary: %+v %v", page, err)
	}
}
func TestAccessFilterValidationAndCursorContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 3; i++ {
		For("serve").Access(AccessEntry{Actor: "phone:a", DeviceID: "a", Operation: "send", Outcome: OK})
	}
	for _, q := range []AccessQuery{{DeviceID: "bad\nID"}, {Operation: "unknown operation"}, {Outcome: "unknown"}, {GroupBy: "name"}} {
		if _, err := ReadAccess(q); err != ErrAccessFilter {
			t.Fatalf("bad filter accepted: %+v %v", q, err)
		}
	}
	first, err := ReadAccess(AccessQuery{DeviceID: "a", Limit: 1})
	if err != nil || first.Next == "" {
		t.Fatal(first, err)
	}
	if _, err := ReadAccess(AccessQuery{DeviceID: "b", Before: first.Next}); err != ErrAccessCursor {
		t.Fatal("cursor crossed device filters", err)
	}
	if _, err := ReadAccess(AccessQuery{DeviceID: "a", GroupBy: "device", Before: first.Next}); err != ErrAccessCursor {
		t.Fatal("cursor crossed view mode", err)
	}
	if p, err := ReadAccess(AccessQuery{DeviceID: "a", Before: first.Next}); err != nil || len(p.Entries) != 2 {
		t.Fatal("same filter cannot continue", p, err)
	}
}
func TestAccessIdentityWithoutDeviceIsNotGuessedFromNameOrIP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, actor := range []string{"user", "menubar"} {
		For("serve").Access(AccessEntry{Actor: actor, Name: "Owner credential", Scope: "master", SourceIP: "203.0.113.8", Operation: "sessions", Outcome: OK})
	}
	p, err := ReadAccess(AccessQuery{GroupBy: "device"})
	if err != nil || len(p.Groups) != 2 {
		t.Fatal(p, err)
	}
	for _, g := range p.Groups {
		if g.DeviceID != "" {
			t.Fatal("invented physical identity")
		}
	}
}
