package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

func TestAccessHistoryCLIUsesSharedPageAndRefusesAmbiguousFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	diag.For("serve").Access(diag.AccessEntry{Actor: "phone:a", Name: "Phone", Operation: "terminal", Outcome: diag.OK, Status: 200})
	output := captureStdout(t, func() {
		if code := cmdLogs([]string{"--access", "--json", "--limit", "1"}); code != 0 {
			t.Fatal(code)
		}
	})
	var page diag.AccessPage
	if json.Unmarshal([]byte(output), &page) != nil || len(page.Entries) != 1 || page.Entries[0].Name != "Phone" {
		t.Fatalf("invalid CLI page: %s", output)
	}
	for _, args := range [][]string{{"--access", "--follow"}, {"--access", "--stats"}, {"--access", "--limit", "999"}, {"--access", "--event", "*"}, {"--before", "bad"}, {"--limit", "100"}} {
		if code := cmdLogs(args); code != 2 {
			t.Fatalf("%v: %d", args, code)
		}
	}
}

func TestAccessCLIShowsDesktopSessionTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sid := "12345678-abcd-1234-abcd-123456789abc"
	diag.For("serve").Access(diag.AccessEntry{Operation: "conversation", SessionID: sid, Outcome: diag.OK})
	output := captureStdout(t, func() {
		if cmdLogs([]string{"--access"}) != 0 {
			t.Fatal("history failed")
		}
	})
	if !strings.Contains(output, "Codex · "+sid) {
		t.Fatal("desktop attribution missing from terminal history")
	}
}

func TestAccessCLIGroupingAndContinuationPreserveFilters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 3; i++ {
		diag.For("serve").Access(diag.AccessEntry{Actor: "phone:a", DeviceID: "id-with-quote'", Name: "Phone", Operation: "send", Outcome: diag.Failed})
	}
	output := captureStdout(t, func() {
		if cmdLogs([]string{"--access", "--device-id", "id-with-quote'", "--operation", "send", "--outcome", "failed", "--group-by", "device", "--limit", "1", "--since", "1h"}) != 0 {
			t.Fatal("grouping failed")
		}
	})
	for _, value := range []string{"--device-id", shellQuote("id-with-quote'"), "--operation 'send'", "--outcome 'failed'", "--group-by 'device'", "--since '"} {
		if !strings.Contains(output, value) {
			t.Fatalf("continuation lost %q: %s", value, output)
		}
	}
	for _, args := range [][]string{{"--group-by", "device"}, {"--access", "--group-by", "name"}, {"--access", "--outcome", "unknown"}} {
		if cmdLogs(args) != 2 {
			t.Fatal("invalid filter accepted", args)
		}
	}
}
