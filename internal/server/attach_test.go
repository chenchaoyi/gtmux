package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/chenchaoyi/gtmux/internal/connect"
)

// The /api/attach scope gate runs BEFORE any PTY is spawned (before the WS upgrade),
// so we can assert it with plain GETs (the real bridge is verified manually on a
// terminal). viewServer wires Share + a guest token but no AttachCommand.

func TestAttach_GuestRefusedNonViewable(t *testing.T) {
	h, _, guest := viewServer(t)
	// Empty view allowlist → a guest cannot attach %1 → refused before upgrade.
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", guest); rr.Code != http.StatusForbidden {
		t.Fatalf("guest attach to non-viewable = %d, want 403 (%s)", rr.Code, rr.Body.String())
	}
}

func TestAttach_GuestViewablePassesGate(t *testing.T) {
	h, share, guest := viewServer(t)
	share.SetConfig(nil, nil, &[]string{"%1"}) // allow %1 for viewing
	// Passes the scope gate; with no AttachCommand wired it then 503s — proving the
	// gate ALLOWED the viewable guest (did not 403).
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", guest); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("viewable guest attach = %d, want 503 (past the gate) (%s)", rr.Code, rr.Body.String())
	}
}

func TestAttach_OwnerNoCommand503(t *testing.T) {
	h, _, _ := viewServer(t)
	if rr := do(t, h, http.MethodGet, "/api/attach?id=%251", testToken); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("owner attach, no AttachCommand = %d, want 503", rr.Code)
	}
}

func TestAttach_MissingID(t *testing.T) {
	h, _, _ := viewServer(t)
	if rr := do(t, h, http.MethodGet, "/api/attach", testToken); rr.Code != http.StatusBadRequest {
		t.Fatalf("attach missing id = %d, want 400", rr.Code)
	}
}

// The tmux client spawned in the PTY MUST get a valid TERM (else "does not support
// clear") AND a UTF-8 locale (else CJK renders as placeholder dashes). Regression guard.
func TestAttachEnv_SetsTermAndUTF8(t *testing.T) {
	env := attachEnv("xterm-256color")
	has := func(want string) bool {
		for _, e := range env {
			if e == want {
				return true
			}
		}
		return false
	}
	if !has("TERM=xterm-256color") {
		t.Errorf("attachEnv must set TERM")
	}
	if !has("LC_CTYPE=en_US.UTF-8") {
		t.Errorf("attachEnv must set a UTF-8 LC_CTYPE (else CJK → dashes)")
	}
}

// sanitizeTerm rejects anything that isn't a terminfo-safe name (a client-supplied
// TERM becomes a spawned process's env var).
func TestSanitizeTerm(t *testing.T) {
	for _, ok := range []string{"xterm-256color", "xterm-ghostty", "screen.xterm-256color", "tmux-256color"} {
		if sanitizeTerm(ok) != ok {
			t.Errorf("sanitizeTerm(%q) should pass", ok)
		}
	}
	for _, bad := range []string{"", "xterm; rm -rf", "a b", "évil", "x\n"} {
		if sanitizeTerm(bad) != "" {
			t.Errorf("sanitizeTerm(%q) should be rejected", bad)
		}
	}
}

// resolveTerm falls back to a safe terminfo when the client TERM is empty or unknown.
func TestResolveTerm_Fallback(t *testing.T) {
	if got := resolveTerm(""); got != "xterm-256color" {
		t.Errorf("empty client term → %q, want xterm-256color", got)
	}
	if got := resolveTerm("definitely-not-a-real-terminfo-xyz"); got != "xterm-256color" {
		t.Errorf("unknown client term → %q, want xterm-256color fallback", got)
	}
}

// Revoking a caller ends its open terminal, not only its next request. auth() checks the
// token once, before the upgrade, and on an isolated serve (2026-10-06) a revoked link's
// and a revoked device's sessions both went on streaming the pane. A real PTY runs a
// printing loop here; the device is revoked mid-stream and the stream must end.
func TestAttach_RevokingTheDeviceEndsItsOpenSession(t *testing.T) {
	testRevokeEnds(t, "while :; do echo tick; sleep 0.05; done")
}

// A terminal with nothing to print is the common case (an agent waiting, a shell at its
// prompt), and there the output pump sits in a blocking read with no data coming. %12's
// re-verification (2026-10-06) found that session still open 4s after the revoke, its
// notice sent: closing the PTY master does not wake that read on macOS.
func TestAttach_RevokingEndsAQuietSession(t *testing.T) {
	testRevokeEnds(t, "echo tick; exec sleep 30")
}

func testRevokeEnds(t *testing.T, script string) {
	t.Helper()
	saved := attachRecheckInterval
	attachRecheckInterval = 100 * time.Millisecond
	t.Cleanup(func() { attachRecheckInterval = saved })

	enroll := NewEnrollManager(nil, nil)
	dev, ok := enroll.Redeem(enroll.Mint(), "probe")
	if !ok {
		t.Fatal("enroll a device")
	}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		AttachCommand: func(string) ([]string, bool) {
			return []string{"/bin/sh", "-c", script}, true
		},
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	open := func(tok string) *websocket.Conn {
		t.Helper()
		c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
			http.Header{"Authorization": {"Bearer " + tok}})
		if err != nil {
			t.Fatalf("attach: %v", err)
		}
		return c
	}
	// stream reads output in the background (a timed-out websocket read breaks the
	// connection for good, so the test never times a read out): it delivers each frame's
	// text, and closes when the stream ends.
	stream := func(c *websocket.Conn) <-chan string {
		ch := make(chan string, 1024)
		go func() {
			defer close(ch)
			for {
				_, data, err := c.ReadMessage()
				if err != nil {
					return
				}
				if op, payload, ok := connect.Decode(data); ok && op == connect.OpOutput {
					ch <- string(payload)
				}
			}
		}()
		return ch
	}
	// read gathers what arrives within d, and whether the stream ended meanwhile.
	read := func(ch <-chan string, d time.Duration) (string, bool) {
		var out strings.Builder
		deadline := time.After(d)
		for {
			select {
			case s, ok := <-ch:
				if !ok {
					return out.String(), true
				}
				out.WriteString(s)
			case <-deadline:
				return out.String(), false
			}
		}
	}

	c := open(dev.Token)
	defer c.Close()
	out := stream(c)
	if got, ended := read(out, 400*time.Millisecond); ended || !strings.Contains(got, "tick") {
		t.Fatalf("before the revoke: ended=%v output=%q, want the stream running", ended, got)
	}
	if !enroll.Revoke(dev.ID) {
		t.Fatal("revoke")
	}
	got, ended := read(out, 3*time.Second)
	if !ended {
		t.Fatal("the session outlived its revoked device")
	}
	if !strings.Contains(got, "access revoked") {
		t.Errorf("the session ended without saying why: %q", got)
	}

	// The serve's own token is never re-checked against the device roster.
	owner := open(testToken)
	defer owner.Close()
	if _, ended := read(stream(owner), 400*time.Millisecond); ended {
		t.Fatal("an owner session ended with no revoke")
	}
}

// A client that leaves a quiet pane takes its tmux client with it. Before, the handler
// closed only the PTY, which does not wake a read on a quiet terminal (macOS): the handler
// and the spawned client stayed until the pane next printed, or forever.
func TestAttach_LeavingAQuietPaneEndsItsTmuxClient(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	enroll := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		AttachCommand: func(string) ([]string, bool) {
			return []string{"/bin/sh", "-c", "echo $$ > " + pidFile + "; echo tick; exec sleep 30"}, true
		},
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
		http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	var pid int
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && pid == 0; time.Sleep(20 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
	}
	if pid == 0 {
		t.Fatal("the attach command never started")
	}
	_ = c.Close() // the client leaves; the pane has nothing more to print
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if syscall.Kill(pid, 0) != nil {
			return // gone, and reaped
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatal("the tmux client outlived the client that left")
}
