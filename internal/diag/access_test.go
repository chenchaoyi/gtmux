package diag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestAccessHistoryIsPagedAndProjectsOnlyAccessMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	logger := For("serve")
	for i := 0; i < 5; i++ {
		logger.Access(AccessEntry{Actor: "phone:known", DeviceID: "known", Name: "My phone", Operation: "send", Method: "POST", Scope: "device", Target: "%21", Status: 200, Outcome: OK})
	}
	logger.Info("unrelated", "PRIVATE CONVERSATION MUST NOT BE EXPOSED")
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 4; i++ {
		page, err := ReadAccess(AccessQuery{Before: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Entries {
			if seen[e.ID] {
				t.Fatalf("pagination repeated %s", e.ID)
			}
			seen[e.ID] = true
			if e.LabelZH != "发送终端输入" || e.DeviceID != "known" || e.Target != "%21" {
				t.Fatalf("incomplete metadata: %+v", e)
			}
		}
		raw, _ := json.Marshal(page)
		if strings.Contains(string(raw), "PRIVATE CONVERSATION") {
			t.Fatal("unrelated diagnostics leaked")
		}
		cursor = page.Next
		if cursor == "" {
			break
		}
	}
	if len(seen) != 5 {
		t.Fatalf("lost access records: %d", len(seen))
	}
	page, err := ReadAccess(AccessQuery{Actor: "another-device"})
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("actor filter: %+v %v", page, err)
	}
	for _, bad := range []string{"not-base64!", encodeAccessCursor("../../config.json", 0), encodeAccessCursor("2026-10-16.jsonl", -1), encodeAccessCursor("1900-01-01.jsonl", 0)} {
		if _, err := ReadAccess(AccessQuery{Before: bad}); err != ErrAccessCursor {
			t.Fatalf("invalid cursor accepted: %s (%v)", bad, err)
		}
	}
}

func TestAccessReaderBudgetHasAContinuableEmptyPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	For("serve").Access(AccessEntry{Actor: "phone:x", Name: "First", Operation: "sessions", Status: 200, Outcome: OK})
	path := ActiveSegment(state.LogsDir(), time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Ordinary diagnostics consume the bounded scan; they must not hide older access.
	_, _ = f.WriteString(strings.Repeat("{\"event\":\"unrelated\"}\n", accessReadBudget/20+10))
	_ = f.Close()
	first, err := ReadAccess(AccessQuery{})
	if err != nil || len(first.Entries) != 0 || first.Next == "" || !first.Partial {
		t.Fatalf("budget page: %+v %v", first, err)
	}
	next, err := ReadAccess(AccessQuery{Before: first.Next})
	if err != nil || len(next.Entries) != 1 {
		t.Fatalf("older record lost at cut boundary: %+v %v", next, err)
	}
	_ = os.Remove(path)
	if _, err := ReadAccess(AccessQuery{Before: first.Next}); err != ErrAccessCursor {
		t.Fatalf("expired cursor: %v", err)
	}
}

func TestAccessRecordRejectsUnknownOperationsAndArbitraryTargets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	For("serve").Access(AccessEntry{Actor: "known", Operation: "secret body", Target: "secret payload", SourceIP: "not an IP", Method: "secret method", Name: "My\nphone", Status: 200})
	page, err := ReadAccess(AccessQuery{})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("missing entry: %+v %v", page, err)
	}
	e := page.Entries[0]
	if e.Operation != "other" || e.Target != "" || e.SourceIP != "" || e.Name != "My phone" || e.Method != "OTHER" {
		t.Fatalf("unsafe metadata: %+v", e)
	}
	files, _ := filepath.Glob(filepath.Join(state.LogsDir(), "*.jsonl"))
	raw, _ := os.ReadFile(files[0])
	if strings.Contains(string(raw), "secret") {
		t.Fatal("unknown operation/target reached storage")
	}
}

func TestAccessHistoryDoesNotFollowLogSymlinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.LogsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "external.jsonl")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(state.LogsDir(), time.Now().Format("2006-01-02")+".jsonl")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAccess(AccessQuery{}); err == nil {
		t.Fatal("symlink became empty or readable history")
	}
}

func TestAccessHistoryRetentionWindowAndActorFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	For("serve").Access(AccessEntry{Actor: "phone:prefix", DeviceID: "full-device-id", Operation: "sessions", Outcome: OK})
	page, err := ReadAccess(AccessQuery{Actor: "full-device-id"})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("full device filter: %+v %v", page, err)
	}
	page, err = ReadAccess(AccessQuery{Since: time.Now().Add(time.Hour)})
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("since boundary ignored: %+v %v", page, err)
	}
}

func TestAccessHistoryRefusesEvenEmptyLinkedDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), state.LogsDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAccess(AccessQuery{}); err == nil {
		t.Fatal("linked log directory became empty history")
	}
}

func TestAccessSessionLocatorIsStructuredAndProjected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	good := "12345678-ABCD-1234-ABCD-123456789ABC"
	for _, id := range []string{good, "private prompt", good + "?token=secret", strings.ReplaceAll(good, "-", "_"), "12345678-abcd-1234-abcd-123456789abg"} {
		For("serve").Access(AccessEntry{Operation: "conversation", SessionID: id, Outcome: OK})
	}
	page, err := ReadAccess(AccessQuery{})
	if err != nil || len(page.Entries) != 5 {
		t.Fatalf("missing records: %+v %v", page, err)
	}
	for i, e := range page.Entries {
		if i == 4 {
			if e.SessionID != strings.ToLower(good) {
				t.Fatal("valid UUID lost")
			}
		} else if e.SessionID != "" {
			t.Fatalf("arbitrary text retained: %+v", e)
		}
	}
	files, _ := filepath.Glob(filepath.Join(state.LogsDir(), "*.jsonl"))
	raw, _ := os.ReadFile(files[0])
	if strings.Contains(string(raw), "private prompt") || strings.Contains(string(raw), "token=secret") {
		t.Fatal("arbitrary query text stored")
	}
}
