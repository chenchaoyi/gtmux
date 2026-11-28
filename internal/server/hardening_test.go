package server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type blockingBadgeRelay struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingBadgeRelay) Send(in PushIntent) error {
	if in.Silent {
		select {
		case b.entered <- struct{}{}:
		default:
		}
		<-b.release
	}
	return nil
}

func TestCHH004_OnTallyAsyncBadgeAndHubSlowTickNonBlocking(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	relay := &blockingBadgeRelay{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	pm := NewPushManager(relay, []DeviceToken{{Token: "tok-1", Origin: originMaster}}, nil, "", nil)

	doneCall := make(chan struct{})
	go func() {
		pm.OnTally(Tally{Waiting: 3})
		close(doneCall)
	}()
	select {
	case <-doneCall:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("OnTally blocked synchronously on badge relay HTTP call")
	}
	<-relay.entered
	close(relay.release)

	// Verify hub.run executes onSlowTick single-flight without blocking tick()/onFastTick()
	// and runs checkServerMode after onSlowTick finishes.
	var statusCalls atomic.Int32
	var fastCalls atomic.Int32
	var slowConcurrent atomic.Int32
	var maxSlowConcurrent atomic.Int32
	slowEntered := make(chan struct{}, 4)
	slowRelease := make(chan struct{})
	modeChecked := make(chan string, 4)

	h := newHub(func() []AgentStatus {
		statusCalls.Add(1)
		return []AgentStatus{{PaneID: "%1", Status: "working"}}
	}, time.Hour, nil)
	h.onFastTick = func() { fastCalls.Add(1) }
	h.onSlowTick = func() {
		cur := slowConcurrent.Add(1)
		for {
			prev := maxSlowConcurrent.Load()
			if cur <= prev || maxSlowConcurrent.CompareAndSwap(prev, cur) {
				break
			}
		}
		select {
		case slowEntered <- struct{}{}:
		default:
		}
		<-slowRelease
		slowConcurrent.Add(-1)
	}
	h.serverModeSig = func() string {
		modeChecked <- "checked"
		return "sig-1"
	}

	// Trigger two overlapping async slow ticks directly and ensure single-flight + non-blocking ticks.
	h.runSlowTickAsync()
	<-slowEntered
	h.runSlowTickAsync() // must be dropped by single-flight atomic.Bool while first is running

	h.tick()
	h.onFastTick()
	if statusCalls.Load() == 0 || fastCalls.Load() == 0 {
		t.Fatal("radar tick or fast tick did not run while slow tick was blocked")
	}
	if maxSlowConcurrent.Load() != 1 {
		t.Fatalf("overlapping slow ticks ran concurrently: max=%d", maxSlowConcurrent.Load())
	}
	close(slowRelease)
	select {
	case <-modeChecked:
	case <-time.After(2 * time.Second):
		t.Fatal("checkServerMode did not run after async onSlowTick completed")
	}
}

func TestCHH005_TranscriptConditionalETagAndPreparedContextFastPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var legacyCalled atomic.Int32
	var conditionalCalls atomic.Int32

	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Transcript: func(id string, earlier int) ([]byte, TranscriptMeta, error) {
			legacyCalled.Add(1)
			return nil, TranscriptMeta{}, nil
		},
		TranscriptConditional: func(id string, earlier, limit int, ifNoneMatch string, includeContext bool) ([]byte, TranscriptMeta, error) {
			conditionalCalls.Add(1)
			meta := TranscriptMeta{
				Etag:            `W/"etag-public-1"`,
				Dropped:         2,
				ContextPrepared: true,
			}
			if includeContext {
				meta.Etag = `W/"etag-reader-1"`
			}
			if ifNoneMatch == meta.Etag {
				return nil, meta, nil
			}
			if includeContext {
				return []byte(`[{"prompt":"hi","segments":[{"context":{"kind":"compacted","title":"Compacted"}}]}]`), meta, nil
			}
			return []byte(`[{"prompt":"hi","segments":[]}]`), meta, nil
		},
	})
	h := s.Handler()

	// Matching If-None-Match short-circuits to 304 while preserving ETag + X-Gtmux-Dropped.
	req304 := httptest.NewRequest(http.MethodGet, "/api/transcript?id=%251", nil)
	req304.Header.Set("Authorization", "Bearer "+testToken)
	req304.Header.Set("If-None-Match", `W/"etag-public-1"`)
	rr304 := httptest.NewRecorder()
	h.ServeHTTP(rr304, req304)
	if rr304.Code != http.StatusNotModified {
		t.Fatalf("conditional GET = %d, want 304", rr304.Code)
	}
	if rr304.Header().Get("ETag") != `W/"etag-public-1"` || rr304.Header().Get("X-Gtmux-Turns-Dropped") != "2" {
		t.Fatalf("304 headers missing metadata: %+v", rr304.Header())
	}

	// Non-matching GET uses ContextPrepared payload without calling legacy Transcript.
	rr200 := do(t, h, http.MethodGet, "/api/transcript?id=%251&context=1", testToken)
	if rr200.Code != http.StatusOK || !strings.Contains(rr200.Body.String(), `"compacted"`) {
		t.Fatalf("context=1 GET = %d %s", rr200.Code, rr200.Body.String())
	}
	if legacyCalled.Load() != 0 || conditionalCalls.Load() != 2 {
		t.Fatalf("legacyCalled=%d conditionalCalls=%d", legacyCalled.Load(), conditionalCalls.Load())
	}
}

func TestCHH006_AttachDeadPeerReapAndCursorSamplerGating(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	origPing, origPong, origIdle := attachPingInterval, attachPongWait, attachIdleCursorWindow
	attachPingInterval = 50 * time.Millisecond
	attachPongWait = 450 * time.Millisecond
	attachIdleCursorWindow = 250 * time.Millisecond
	t.Cleanup(func() {
		attachPingInterval = origPing
		attachPongWait = origPong
		attachIdleCursorWindow = origIdle
	})

	var pidSeq atomic.Int32
	pidDir := t.TempDir()
	var cursorCalls atomic.Int32

	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		AttachCommand: func(string) ([]string, bool) {
			n := pidSeq.Add(1)
			pf := filepath.Join(pidDir, fmt.Sprintf("pty-%d.pid", n))
			return []string{"/bin/sh", "-c", fmt.Sprintf("echo $$ > %q; exec sleep 30", pf)}, true
		},
		AttachCursor: func(string) (int, int, bool, bool) {
			cursorCalls.Add(1)
			return 1, 1, false, true
		},
	})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	wsBase := strings.Replace(ts.URL, "http", "ws", 1)

	readPID := func(n int) int {
		var pid int
		pf := filepath.Join(pidDir, fmt.Sprintf("pty-%d.pid", n))
		waitFor(t, func() bool {
			raw, err := os.ReadFile(pf)
			if err != nil {
				return false
			}
			pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
			return err == nil && pid > 1
		})
		return pid
	}

	// 1) predict=0 opt-out never invokes AttachCursor.
	c0, _, err := websocket.DefaultDialer.Dial(wsBase+"/api/attach?id=%251&predict=0",
		http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatal(err)
	}
	pid0 := readPID(1)
	time.Sleep(200 * time.Millisecond)
	_ = c0.Close()
	waitFor(t, func() bool { return syscall.Kill(pid0, 0) != nil })
	if got := cursorCalls.Load(); got != 0 {
		t.Fatalf("AttachCursor called %d times with predict=0, want 0", got)
	}

	// 2) predict=1 samples while active, then reaps dead peer when pongs stop on a quiet pane.
	c1, _, err := websocket.DefaultDialer.Dial(wsBase+"/api/attach?id=%251&predict=1",
		http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatal(err)
	}
	pid1 := readPID(2)
	// Suppress automatic Pong replies so missed-pong deadline expires.
	c1.SetPingHandler(func(string) error { return nil })
	go func() {
		for {
			if _, _, rerr := c1.ReadMessage(); rerr != nil {
				return
			}
		}
	}()
	waitFor(t, func() bool { return cursorCalls.Load() > 0 })

	// Wait for missed-pong read deadline to close session and reap the quiet PTY child process.
	waitFor(t, func() bool {
		return syscall.Kill(pid1, 0) != nil
	})
	_ = c1.Close()
}

func TestCHH007_SSERevocationGuestRedactionBodyCapsAndAPNsCleanup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager(nil, nil)
	dev, ok := enroll.Redeem(enroll.Mint(), "Test iPhone")
	if !ok {
		t.Fatal("Redeem failed")
	}
	share := NewShareManager(ShareState{}, nil)
	share.OnBroadcast(enroll.BroadcastGuestScopes)
	on := true
	share.SetConfig(&on, &[]string{"%1"}, nil)
	guest := enroll.MintGuest("g", []string{"%1"}, nil, 0).Token

	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		Share:  share,
		AgentsJSON: func() ([]byte, error) {
			return []byte(`[{"pane_id":"%1","agent":"Codex","cwd":"/Users/secret/project","icon":"/Users/secret/App.app","launch_host":"iTerm2"}]`), nil
		},
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// 1) /api/addresses rejects guest with 403, allows enrolled device with 200.
	if rr := do(t, s.Handler(), http.MethodGet, "/api/addresses", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest GET /api/addresses = %d, want 403", rr.Code)
	}
	if rr := do(t, s.Handler(), http.MethodGet, "/api/addresses", dev.Token); rr.Code != http.StatusOK {
		t.Fatalf("device GET /api/addresses = %d, want 200", rr.Code)
	}

	// 2) Guest /api/agents redacts cwd, icon, and launch_host.
	rrGuest := do(t, s.Handler(), http.MethodGet, "/api/agents", guest)
	if rrGuest.Code != http.StatusOK {
		t.Fatalf("guest /api/agents = %d", rrGuest.Code)
	}
	bodyStr := rrGuest.Body.String()
	for _, forbidden := range []string{"cwd", "/Users/secret", "icon", "launch_host", "iTerm2"} {
		if strings.Contains(bodyStr, forbidden) {
			t.Fatalf("guest /api/agents leaked %q in %s", forbidden, bodyStr)
		}
	}

	// 3) Oversized JSON POST (>64 KiB) and >512B push token are rejected with 400.
	pm := NewPushManager(&fakeRelay{}, nil, nil, "", nil)
	sWithPush := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{Push: pm, Enroll: enroll})
	hugeBody := `{"token":"` + strings.Repeat("x", 70*1024) + `"}`
	reqHuge := httptest.NewRequest(http.MethodPost, "/api/push/register", strings.NewReader(hugeBody))
	reqHuge.Header.Set("Authorization", "Bearer "+testToken)
	rrHuge := httptest.NewRecorder()
	sWithPush.Handler().ServeHTTP(rrHuge, reqHuge)
	if rrHuge.Code != http.StatusBadRequest {
		t.Fatalf("oversized /api/push/register = %d, want 400", rrHuge.Code)
	}

	longTokBody := `{"token":"` + strings.Repeat("a", 600) + `"}`
	reqLong := httptest.NewRequest(http.MethodPost, "/api/push/register", strings.NewReader(longTokBody))
	reqLong.Header.Set("Authorization", "Bearer "+testToken)
	rrLong := httptest.NewRecorder()
	sWithPush.Handler().ServeHTTP(rrLong, reqLong)
	if rrLong.Code != http.StatusBadRequest {
		t.Fatalf(">512B token /api/push/register = %d, want 400", rrLong.Code)
	}

	// 4) SSE stream closes promptly on next event after device revocation.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reqSSE, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	reqSSE.Header.Set("Authorization", "Bearer "+dev.Token)
	respSSE, err := http.DefaultClient.Do(reqSSE)
	if err != nil {
		t.Fatal(err)
	}
	defer respSSE.Body.Close()
	reader := bufio.NewReader(respSSE.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("initial SSE frame err=%v", err)
		}
		if line == "\n" || line == "\r\n" {
			break
		}
	}
	enroll.Revoke(dev.ID)
	s.hub.broadcast(sseEvent{name: "agents", data: []byte(`{"rev":99}`)})
	for {
		line, readErr := reader.ReadString('\n')
		if strings.Contains(line, `"rev":99`) {
			t.Fatalf("revoked device still received SSE event: %q", line)
		}
		if readErr != nil {
			break
		}
	}

	// 5) APNs 410 / 400 BadDeviceToken retires iOS token; 400 TopicDisallowed retains it;
	// and native CollapseID > 64 bytes is hashed <= 64 ASCII bytes.
	for _, tc := range []struct {
		name   string
		status int
		detail string
		want   int
	}{
		{"410-unregistered", 410, `{"reason":"Unregistered"}`, 0},
		{"400-bad-token", 400, `{"reason":"BadDeviceToken"}`, 0},
		{"400-topic-disallowed", 400, `{"reason":"TopicDisallowed"}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apnsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusBadGateway, map[string]any{
					"error":  "apns",
					"status": tc.status,
					"detail": tc.detail,
				})
			}))
			defer apnsSrv.Close()
			p := NewPushManager(NewHTTPRelay(apnsSrv.URL, "k"), nil, nil, "Mac", nil)
			p.Register(DeviceToken{Token: "ios-tok-1", Platform: "ios", Origin: originMaster})
			p.Test()
			if got := len(p.Tokens()); got != tc.want {
				t.Fatalf("%s: tokens left = %d, want %d", tc.name, got, tc.want)
			}
		})
	}

	cid := alertCollapseID(Alert{
		Kind:      "waiting",
		HomeID:    strings.Repeat("a", 32),
		SessionID: strings.Repeat("b", 64),
	})
	if len(cid) > 64 {
		t.Fatalf("alertCollapseID len = %d (> 64 APNs limit): %q", len(cid), cid)
	}
}
