package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/chenchaoyi/gtmux/internal/connect"
)

// The bridge honors the client's PAUSE/RESUME (remote-terminal-client). Until 2026-10-06
// the handler ignored both while the spec said it honored them (%12's review of #1432);
// these pin what honoring them means, at the edges HQ named: what is held, what keeps
// moving, and what still ends a paused session.

// flowSession is one attach session for these tests: its client, its output, and
// whether the handler has returned.
type flowSession struct {
	t    *testing.T
	c    *websocket.Conn
	out  chan string
	done chan struct{}
}

func openFlow(t *testing.T, script string, deps func(*Deps), token string) *flowSession {
	t.Helper()
	d := Deps{AttachCommand: func(string) ([]string, bool) { return []string{"/bin/sh", "-c", script}, true }}
	if deps != nil {
		deps(&d)
	}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, d)
	fs := &flowSession{t: t, out: make(chan string, 4096), done: make(chan struct{})}
	inner := s.Handler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
		if r.URL.Path == "/api/attach" {
			close(fs.done)
		}
	}))
	t.Cleanup(ts.Close)
	if token == "" {
		token = testToken
	}
	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
		http.Header{"Authorization": {"Bearer " + token}})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	fs.c = c
	go func() {
		defer close(fs.out)
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if op, payload, ok := connect.Decode(data); ok && op == connect.OpOutput {
				fs.out <- string(payload)
			}
		}
	}()
	return fs
}

func (fs *flowSession) send(op byte, payload string) {
	fs.t.Helper()
	if err := fs.c.WriteMessage(websocket.BinaryMessage, connect.Encode(op, []byte(payload))); err != nil {
		fs.t.Fatalf("send %q: %v", op, err)
	}
}

// read gathers output for d, or until want appears; ended reports the stream closing.
func (fs *flowSession) read(d time.Duration, want string) (got string, ended bool) {
	var b strings.Builder
	deadline := time.After(d)
	for {
		select {
		case s, ok := <-fs.out:
			if !ok {
				return b.String(), true
			}
			b.WriteString(s)
			if want != "" && strings.Contains(b.String(), want) {
				return b.String(), false
			}
		case <-deadline:
			return b.String(), false
		}
	}
}

func (fs *flowSession) handlerEnds(d time.Duration) bool {
	select {
	case <-fs.done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestAttach_PauseHoldsOutputUntilResume(t *testing.T) {
	fs := openFlow(t, "exec cat", nil, "")
	fs.send(connect.OpResume, "") // a RESUME with nothing paused does nothing
	fs.send(connect.OpPause, "")
	fs.send(connect.OpInput, "mark-1\n")
	if got, _ := fs.read(400*time.Millisecond, "mark-1"); strings.Contains(got, "mark-1") {
		t.Fatalf("output arrived while paused: %q", got)
	}
	fs.send(connect.OpResume, "")
	if got, _ := fs.read(3*time.Second, "mark-1"); !strings.Contains(got, "mark-1") {
		t.Fatalf("RESUME did not release the held output: %q", got)
	}
	// Repeated frames are idempotent: two PAUSEs need one RESUME, and a second RESUME
	// changes nothing.
	fs.send(connect.OpPause, "")
	fs.send(connect.OpPause, "")
	fs.send(connect.OpInput, "mark-2\n")
	if got, _ := fs.read(400*time.Millisecond, "mark-2"); strings.Contains(got, "mark-2") {
		t.Fatalf("output arrived while paused twice: %q", got)
	}
	fs.send(connect.OpResume, "")
	fs.send(connect.OpResume, "")
	if got, _ := fs.read(3*time.Second, "mark-2"); !strings.Contains(got, "mark-2") {
		t.Fatalf("output held after RESUME: %q", got)
	}
	fs.send(connect.OpInput, "mark-3\n")
	if got, _ := fs.read(3*time.Second, "mark-3"); !strings.Contains(got, "mark-3") {
		t.Fatalf("the stream did not flow after the second RESUME: %q", got)
	}
}

// Pausing holds the output, never the input: what the reader types while paused reaches
// the program at once.
func TestAttach_InputReachesTheProgramWhilePaused(t *testing.T) {
	f := filepath.Join(t.TempDir(), "got")
	fs := openFlow(t, `read x; printf '%s' "$x" > `+f+`; exec cat`, nil, "")
	fs.send(connect.OpPause, "")
	fs.send(connect.OpInput, "hello\n")
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if b, _ := os.ReadFile(f); string(b) == "hello" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("input sent while paused did not reach the program")
		}
	}
	if got, _ := fs.read(200*time.Millisecond, "hello"); got != "" {
		t.Errorf("output arrived while paused: %q", got)
	}
}

// Nothing is lost across a pause: a long run of output resumes byte for byte.
func TestAttach_OutputResumesWhole(t *testing.T) {
	const n = 30000
	fs := openFlow(t, "read x; seq 1 "+strconv.Itoa(n)+"; exec cat", nil, "")
	fs.send(connect.OpPause, "")
	fs.send(connect.OpInput, "go\n")
	if got, _ := fs.read(400*time.Millisecond, ""); got != "" {
		t.Fatalf("output arrived while paused: %.80q", got)
	}
	fs.send(connect.OpResume, "")
	got, _ := fs.read(10*time.Second, "\n"+strconv.Itoa(n)+"\r\n")
	lines := strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n")
	start := -1
	for i, l := range lines {
		if l == "1" {
			start = i
			break
		}
	}
	if start < 0 || len(lines) < start+n {
		t.Fatalf("got %d lines, want the run 1..%d", len(lines), n)
	}
	for i := 0; i < n; i++ {
		if lines[start+i] != strconv.Itoa(i+1) {
			t.Fatalf("line %d is %q, want %d: output was lost or reordered across the pause", i, lines[start+i], i+1)
		}
	}
}

// A paused session still ends, whichever side ends it.
func TestAttach_APausedSessionStillEnds(t *testing.T) {
	t.Run("revoked", func(t *testing.T) {
		saved := attachRecheckInterval
		attachRecheckInterval = 20 * time.Millisecond
		t.Cleanup(func() { attachRecheckInterval = saved })
		enroll := NewEnrollManager(nil, nil)
		dev, ok := enroll.Redeem(enroll.Mint(), "probe")
		if !ok {
			t.Fatal("enroll a device")
		}
		fs := openFlow(t, "exec yes paused", func(d *Deps) { d.Enroll = enroll }, dev.Token)
		if _, ended := fs.read(300*time.Millisecond, "paused"); ended {
			t.Fatal("stream ended early")
		}
		fs.send(connect.OpPause, "")
		time.Sleep(100 * time.Millisecond)
		if !enroll.Revoke(dev.ID) {
			t.Fatal("revoke")
		}
		if !fs.handlerEnds(3 * time.Second) {
			t.Fatal("a paused session outlived its revoked device")
		}
	})
	t.Run("client leaves", func(t *testing.T) {
		fs := openFlow(t, "exec yes paused", nil, "")
		fs.read(300*time.Millisecond, "paused")
		fs.send(connect.OpPause, "")
		time.Sleep(100 * time.Millisecond)
		_ = fs.c.Close()
		if !fs.handlerEnds(3 * time.Second) {
			t.Fatal("a paused session outlived its client")
		}
	})
	t.Run("program exits, then RESUME", func(t *testing.T) {
		fs := openFlow(t, `read x; echo "bye-$x"`, nil, "")
		fs.send(connect.OpPause, "")
		fs.send(connect.OpInput, "z\n")
		if got, ended := fs.read(400*time.Millisecond, "bye-z"); ended || strings.Contains(got, "bye-z") {
			t.Fatalf("while paused: ended=%v output=%q, want neither", ended, got)
		}
		fs.send(connect.OpResume, "")
		got, _ := fs.read(3*time.Second, "bye-z")
		if !strings.Contains(got, "bye-z") {
			t.Fatalf("the program's last output was lost: %q", got)
		}
		if !fs.handlerEnds(3 * time.Second) {
			t.Fatal("the session did not end after its program exited and the stream resumed")
		}
	})
}

// A flooding pane, a pause and input together do not wedge the session: frames keep
// being read (the RESUME below is one), so the stream comes back.
func TestAttach_FloodPauseAndInputDoNotDeadlock(t *testing.T) {
	fs := openFlow(t, "yes flood & exec cat > /dev/null", nil, "")
	if got, _ := fs.read(300*time.Millisecond, "flood"); !strings.Contains(got, "flood") {
		t.Fatalf("no flood: %q", got)
	}
	fs.send(connect.OpPause, "")
	for i := 0; i < 200; i++ {
		fs.send(connect.OpInput, "typed while paused\n")
	}
	fs.read(300*time.Millisecond, "") // drain what was in flight
	if got, _ := fs.read(300*time.Millisecond, ""); got != "" {
		t.Fatalf("output still flowing while paused: %.80q", got)
	}
	fs.send(connect.OpResume, "")
	if got, _ := fs.read(3*time.Second, "flood"); !strings.Contains(got, "flood") {
		t.Fatal("the stream did not come back after RESUME")
	}
}
