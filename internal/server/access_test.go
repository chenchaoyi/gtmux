package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

func TestAccessHistoryUsesVerifiedDeviceAndGuardsOwnerProjection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager([]EnrolledDevice{
		{ID: "phone-id", Token: "paired-secret", Name: "Personal iPhone", Platform: "iOS 26", Scope: scopeDevice},
		{ID: "guest-id", Token: "guest-secret", Name: "Guest", Scope: scopeGuest},
	}, nil)
	s := New(Config{Token: "master-secret"}, Deps{Enroll: enroll, AgentsJSON: func() ([]byte, error) { return []byte(`[]`), nil }, Send: func(string, string, string, bool, string) error { return nil }})
	h := s.Handler()
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "203.0.113.8:4321"
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Forwarded-For", "198.51.100.9")
		req.Header.Set("X-Gtmux-Actor", "hq")
		req.Header.Set("X-Gtmux-Client", "iOS 26")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	for i := 0; i < 20; i++ {
		if rr := call("GET", "/api/agents?token=QUERY-SECRET", "paired-secret", ""); rr.Code != 200 {
			t.Fatal(rr.Code)
		}
	}
	for i := 0; i < 2; i++ {
		if rr := call("POST", "/api/send", "paired-secret", `{"id":"%21","text":"PRIVATE COMMAND"}`); rr.Code != 200 {
			t.Fatal(rr.Code, rr.Body.String())
		}
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 3 {
		t.Fatalf("reads not coalesced or writes lost: %d %v", len(page.Entries), err)
	}
	for _, e := range page.Entries {
		if e.Operation == "send" && e.Target != "%21" {
			t.Fatal("send pane locator missing")
		}
		if e.DeviceID != "phone-id" || e.Name != "Personal iPhone" || e.Actor != "phone:phone-id" || e.SourceIP != "203.0.113.8" {
			t.Fatalf("spoofed or lost identity: %+v", e)
		}
	}
	if rr := call("GET", "/api/access-history", "guest-secret", ""); rr.Code != 403 || strings.Contains(rr.Body.String(), "Personal iPhone") {
		t.Fatal("guest could read history")
	}
	if rr := call("GET", "/api/access-history", "unknown", ""); rr.Code != 401 {
		t.Fatal("unknown credential accepted")
	}
	rr := call("GET", "/api/access-history?limit=2", "paired-secret", "")
	if rr.Code != 200 || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(rr.Code)
	}
	var history diag.AccessPage
	if json.Unmarshal(rr.Body.Bytes(), &history) != nil || len(history.Entries) != 2 || history.Next == "" {
		t.Fatalf("invalid history page: %s", rr.Body.String())
	}
	raw := todaysLog(t)
	for _, secret := range []string{"paired-secret", "master-secret", "QUERY-SECRET", "PRIVATE COMMAND"} {
		// Existing act.send uses a hash and length, not the input either.
		if strings.Contains(raw, secret) {
			t.Fatalf("secret/payload in audit store: %s", secret)
		}
	}
	if call("GET", "/api/access-history?limit=999", "paired-secret", "").Code != 400 {
		t.Fatal("unbounded history accepted")
	}
	if call("GET", "/api/access-history?before=invalid", "paired-secret", "").Code != 410 {
		t.Fatal("invalid cursor was not explicit")
	}
}

func TestAccessReadCoalescingIsPerServerIdentityAndExpires(t *testing.T) {
	var reads accessReads
	now := time.Now()
	if !reads.keep("device-a/chat/%1", now) || reads.keep("device-a/chat/%1", now.Add(time.Minute)) || !reads.keep("device-b/chat/%1", now) || !reads.keep("device-a/chat/%2", now) || !reads.keep("device-a/chat/%1", now.Add(5*time.Minute)) {
		t.Fatal("invalid read coalescing scope")
	}
}

func TestAccessStreamKeepsFlusherAndRecordsActualBoundaries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "master"}, Deps{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer master")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !rr.Flushed {
		t.Fatal("SSE writer lost Flusher")
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 2 || page.Entries[0].Operation != "live_close" || page.Entries[1].Operation != "live_open" {
		t.Fatalf("stream lifecycle not recorded: %+v %v", page, err)
	}
}

func TestAccessFailedStreamOpenIsNotLost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "master"}, Deps{})
	req := httptest.NewRequest("GET", "/api/attach?id=%2521", nil)
	req.Header.Set("Authorization", "Bearer master")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatal(rr.Code)
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Operation != "attach_open" || page.Entries[0].Status != 503 || page.Entries[0].Outcome != diag.Failed {
		t.Fatalf("missing failed opening: %+v %v", page, err)
	}
}

func TestAccessReadErrorsRemainIndividual(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "master"}, Deps{AgentsJSON: func() ([]byte, error) { return nil, fmt.Errorf("unavailable") }})
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.Header.Set("Authorization", "Bearer master")
		s.Handler().ServeHTTP(httptest.NewRecorder(), req)
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 2 {
		t.Fatalf("read failures coalesced: %+v %v", page, err)
	}
}

func TestAccessSuccessfulPairingRecordsIdentityNotCode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager(nil, nil)
	code := enroll.Mint()
	s := New(Config{Token: "master"}, Deps{Enroll: enroll})
	req := httptest.NewRequest("POST", "/api/enroll", strings.NewReader(`{"enrollCode":"`+code+`","name":"My iPad"}`))
	req.Header.Set("X-Gtmux-Client", "iOS 26")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	page, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Operation != "pair" || page.Entries[0].DeviceID == "" || page.Entries[0].Name != "My iPad" {
		t.Fatalf("pair receipt: %+v %v", page, err)
	}
	if strings.Contains(todaysLog(t), code) {
		t.Fatal("redeemed code reached audit")
	}
}

func TestAccessWebSocketLifecycleAndPTYStartFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			fs := openFlow(t, "printf audit-ready", func(d *Deps) {
				if failed {
					d.AttachCommand = func(string) ([]string, bool) { return []string{"/gtmux-nonexistent-test-program"}, true }
				}
			}, "")
			_, _ = fs.read(time.Second, "")
			if !fs.handlerEnds(time.Second) {
				t.Fatal("isolated stream did not close")
			}
			page, err := diag.ReadAccess(diag.AccessQuery{})
			if err != nil {
				t.Fatal(err)
			}
			if failed {
				if len(page.Entries) != 1 || page.Entries[0].Outcome != diag.Failed || page.Entries[0].Status != 101 {
					t.Fatalf("PTY failure after upgrade lost: %+v", page)
				}
			} else if len(page.Entries) != 2 || page.Entries[0].Operation != "attach_close" || page.Entries[1].Operation != "attach_open" {
				t.Fatalf("WebSocket lifecycle: %+v", page)
			}
		})
	}
}
