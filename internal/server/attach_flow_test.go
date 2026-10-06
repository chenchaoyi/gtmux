package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// A program that ends while its session is paused ends the session: it is reaped, its
// last output is delivered and the handler returns, with no RESUME. It used to stay
// open with the program defunct until the client resumed or left (%12, 2026-10-06).
// Echo is off, so the read already waiting when PAUSE arrives takes the program's last
// output and the terminal is empty when it exits: the case where the server holds it.
func TestAttach_AProgramThatEndsWhilePausedEndsTheSession(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	fs := openFlow(t, `stty -echo; echo $$ > `+pidFile+`; read x; printf FINAL; exit 0`, nil, "")
	var pid string
	for deadline := time.Now().Add(3 * time.Second); pid == ""; time.Sleep(10 * time.Millisecond) {
		b, _ := os.ReadFile(pidFile)
		pid = strings.TrimSpace(string(b))
		if time.Now().After(deadline) {
			t.Fatal("the program never started")
		}
	}
	fs.send(connect.OpPause, "")
	fs.send(connect.OpInput, "go\n")
	got, ended := fs.read(3*time.Second, "FINAL")
	if !strings.Contains(got, "FINAL") {
		t.Fatalf("the program's last output was not delivered: %q (ended=%v)", got, ended)
	}
	if !fs.handlerEnds(3 * time.Second) {
		t.Fatal("the session outlived its program")
	}
	if out, _ := exec.Command("ps", "-o", "stat=", "-p", pid).Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("the program is still there: ps stat %q", strings.TrimSpace(string(out)))
	}
}

// gateListener holds every server write while its gate is shut, until released: a
// client that has stopped reading, for exactly as long as a test wants.
type gateListener struct {
	net.Listener
	mu      sync.Mutex
	gate    chan struct{}
	blocked atomic.Int32
}

type gateConn struct {
	net.Conn
	l *gateListener
}

func (l *gateListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &gateConn{Conn: c, l: l}, nil
}

func (l *gateListener) shut() { l.mu.Lock(); l.gate = make(chan struct{}); l.mu.Unlock() }
func (l *gateListener) open() {
	l.mu.Lock()
	if l.gate != nil {
		close(l.gate)
		l.gate = nil
	}
	l.mu.Unlock()
}

func (c *gateConn) Write(b []byte) (int, error) {
	c.l.mu.Lock()
	g := c.l.gate
	c.l.mu.Unlock()
	if g != nil {
		c.l.blocked.Add(1)
		<-g
	}
	return c.Conn.Write(b)
}

// A PAUSE read while output waits for the write lock holds that output. The cursor
// sampler holds the lock inside a write the client is not draining; the program's output
// is read and waits for the lock; PAUSE is read (an input after it has reached the
// program); the write lock comes free. The output used to be sent then, because the
// pause was checked before the lock was taken (%12, 2026-10-06); it now waits for
// RESUME, and arrives whole after it.
func TestAttach_APauseWhileOutputWaitsForTheLockHoldsIt(t *testing.T) {
	ack := filepath.Join(t.TempDir(), "ack")
	var cursorX atomic.Int32
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		AttachCommand: func(string) ([]string, bool) {
			return []string{"/bin/sh", "-c", `stty -echo; while read x; do echo "OUT:$x"; echo "$x" >> ` + ack + `; done`}, true
		},
		AttachCursor: func(string) (int, int, bool, bool) { return int(cursorX.Load()), 0, false, true },
	})
	ts := httptest.NewUnstartedServer(s.Handler())
	gl := &gateListener{Listener: ts.Listener}
	ts.Listener = gl
	ts.Start()
	t.Cleanup(func() {
		gl.open()
		ts.Close()
	})
	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
		http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer c.Close()
	out := make(chan string, 1024)
	go func() {
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				close(out)
				return
			}
			if op, payload, ok := connect.Decode(data); ok && op == connect.OpOutput {
				out <- string(payload)
			}
		}
	}()
	time.Sleep(300 * time.Millisecond) // the first cursor frame goes out
	gl.shut()
	cursorX.Store(7) // the next sample writes, takes the lock, and stays inside the write
	for deadline := time.Now().Add(3 * time.Second); gl.blocked.Load() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the cursor sampler never reached a held write")
		}
	}
	send := func(op byte, p string) {
		if err := c.WriteMessage(websocket.BinaryMessage, connect.Encode(op, []byte(p))); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	send(connect.OpInput, "before\n")
	time.Sleep(300 * time.Millisecond) // OUT:before is read and waits for the lock
	send(connect.OpPause, "")
	send(connect.OpInput, "after\n") // reaches the program only once PAUSE has been read
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if b, _ := os.ReadFile(ack); strings.Contains(string(b), "after") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the input after PAUSE never reached the program")
		}
	}
	gl.open() // the cursor write completes and the lock comes free
	var got strings.Builder
	for deadline := time.After(500 * time.Millisecond); ; {
		select {
		case s, ok := <-out:
			if !ok {
				t.Fatal("the stream ended")
			}
			got.WriteString(s)
			continue
		case <-deadline:
		}
		break
	}
	if strings.Contains(got.String(), "OUT:") {
		t.Fatalf("output was sent after PAUSE: %q", got.String())
	}
	send(connect.OpResume, "")
	for deadline := time.After(3 * time.Second); !strings.Contains(got.String(), "OUT:after"); {
		select {
		case s, ok := <-out:
			if !ok {
				t.Fatalf("the stream ended: %q", got.String())
			}
			got.WriteString(s)
		case <-deadline:
			t.Fatalf("held output did not arrive after RESUME: %q", got.String())
		}
	}
	if !strings.Contains(got.String(), "OUT:before") {
		t.Errorf("the held output was lost: %q", got.String())
	}
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
