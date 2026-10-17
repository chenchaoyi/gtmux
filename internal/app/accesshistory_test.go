package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

func TestAccessHistoryCLIUsesSharedPageAndRefusesAmbiguousFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	diag.For("serve").Access(diag.AccessEntry{Origin: diag.AccessExternal, Actor: "phone:a", Name: "Phone", Operation: "terminal", Outcome: diag.OK, Status: 200})
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
	diag.For("serve").Access(diag.AccessEntry{Origin: diag.AccessExternal, Operation: "conversation", SessionID: sid, Outcome: diag.OK})
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
		diag.For("serve").Access(diag.AccessEntry{Origin: diag.AccessExternal, Actor: "phone:a", DeviceID: "id-with-quote'", Name: "Phone", Operation: "send", Outcome: diag.Failed})
	}
	output := captureStdout(t, func() {
		if cmdLogs([]string{"--access", "--device-id", "id-with-quote'", "--operation", "send", "--outcome", "failed", "--group-by", "device", "--limit", "1", "--since", "1h"}) != 0 {
			t.Fatal("grouping failed")
		}
	})
	for _, value := range []string{"--origin 'external'", "--device-id", shellQuote("id-with-quote'"), "--operation 'send'", "--outcome 'failed'", "--group-by 'device'", "--since '"} {
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

func TestAccessCLIOriginRangesKeepInternalDiagnosticsAvailable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, origin := range []string{diag.AccessExternal, diag.AccessLocal, diag.AccessUnknown} {
		diag.For("serve").Access(diag.AccessEntry{Origin: origin, Actor: "owner", Operation: "settings", Outcome: diag.OK})
	}
	for _, tc := range []struct {
		origin string
		count  int
	}{{"", 1}, {"external", 1}, {"local", 1}, {"unknown", 1}, {"all", 3}} {
		args := []string{"--access", "--json"}
		if tc.origin != "" {
			args = append(args, "--origin", tc.origin)
		}
		output := captureStdout(t, func() {
			if cmdLogs(args) != 0 {
				t.Fatal(args)
			}
		})
		var page diag.AccessPage
		if json.Unmarshal([]byte(output), &page) != nil || len(page.Entries) != tc.count {
			t.Fatal(tc.origin, output)
		}
	}
	for _, args := range [][]string{{"--origin", "all"}, {"--access", "--origin", "invalid"}} {
		if cmdLogs(args) != 2 {
			t.Fatal("invalid origin flags accepted", args)
		}
	}
}

func TestDoctorLocalServeReadsDeclareInternalActor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Intercept only this test's HTTP client, without binding the real serve port.
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	called := false
	http.DefaultTransport = accessTestTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		if r.Header.Get("X-Gtmux-Actor") != "system" || r.URL.Host != "127.0.0.1:8765" {
			t.Errorf("doctor request is not labelled local: %s %s", r.URL.Host, r.Header.Get("X-Gtmux-Actor"))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	if _, err := localServeGet("/api/push/tokens"); err != nil || !called {
		t.Fatal(called, err)
	}
}

type accessTestTransport func(*http.Request) (*http.Response, error)

func (f accessTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
