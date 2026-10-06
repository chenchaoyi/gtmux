package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/chenchaoyi/gtmux/internal/connect"
)

// stallListener hands out connections whose writes block, once stall is set, until the
// connection is closed: a client that has stopped reading, as the server sees it.
type stallListener struct {
	net.Listener
	stall   atomic.Bool
	blocked atomic.Int32
	mu      sync.Mutex
	conns   []*stallConn
}

type stallConn struct {
	net.Conn
	l      *stallListener
	closed chan struct{}
	once   sync.Once
}

func (l *stallListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	sc := &stallConn{Conn: c, l: l, closed: make(chan struct{})}
	l.mu.Lock()
	l.conns = append(l.conns, sc)
	l.mu.Unlock()
	return sc, nil
}

func (c *stallConn) Write(b []byte) (int, error) {
	if c.l.stall.Load() {
		c.l.blocked.Add(1)
		<-c.closed
		return 0, net.ErrClosed
	}
	return c.Conn.Write(b)
}

func (c *stallConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func (l *stallListener) closeAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.conns {
		_ = c.Close()
	}
}

// Revoking a device ends its session even while the output pump is stuck in a write the
// client is not draining. The revoke path used to take the pump's write lock to send its
// notice first, so it waited behind that write for as long as the client stalled (%12's
// reproduction, 2026-10-06). The notice is now bounded and best effort; the end is not.
func TestAttach_RevokeEndsASessionStuckInAWrite(t *testing.T) {
	savedRecheck, savedNotice := attachRecheckInterval, attachNoticeWait
	attachRecheckInterval, attachNoticeWait = 20*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { attachRecheckInterval, attachNoticeWait = savedRecheck, savedNotice })

	enroll := NewEnrollManager(nil, nil)
	dev, ok := enroll.Redeem(enroll.Mint(), "probe")
	if !ok {
		t.Fatal("enroll a device")
	}
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		AttachCommand: func(string) ([]string, bool) {
			return []string{"/bin/sh", "-c", "exec yes stalled-output"}, true
		},
	})
	handlerDone := make(chan struct{})
	inner := s.Handler()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
		if r.URL.Path == "/api/attach" {
			close(handlerDone)
		}
	}))
	sl := &stallListener{Listener: ts.Listener}
	ts.Listener = sl
	ts.Start()
	t.Cleanup(func() {
		sl.closeAll() // releases a write still stuck if the handler never ended
		ts.Close()
	})

	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
		http.Header{"Authorization": {"Bearer " + dev.Token}})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer c.Close()
	if _, _, err := c.ReadMessage(); err != nil { // the stream is running
		t.Fatalf("first frame: %v", err)
	}
	sl.stall.Store(true)
	for deadline := time.Now().Add(3 * time.Second); sl.blocked.Load() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the output pump never reached a stalled write")
		}
	}
	if !enroll.Revoke(dev.ID) {
		t.Fatal("revoke")
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("a revoked session stayed open behind a write its client was not draining")
	}
}

// A client that does not consume holds the PTY read back: the pump blocks in one write
// and attempts no other, so nothing queues behind it. PAUSE and RESUME are reserved and
// ignored: sending them neither ends the session nor changes the stream (the spec's
// flow-control requirement, which once promised they paused the read; no client sent
// them and the server never acted on them).
func TestAttach_BackpressureHoldsThePTYReadAndFlowFramesAreIgnored(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll: enroll,
		AttachCommand: func(string) ([]string, bool) {
			return []string{"/bin/sh", "-c", "exec yes flood"}, true
		},
	})
	ts := httptest.NewUnstartedServer(s.Handler())
	sl := &stallListener{Listener: ts.Listener}
	ts.Listener = sl
	ts.Start()
	t.Cleanup(func() {
		sl.closeAll()
		ts.Close()
	})
	c, _, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/api/attach?id=%251",
		http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer c.Close()
	for _, op := range []byte{connect.OpPause, connect.OpResume} {
		if err := c.WriteMessage(websocket.BinaryMessage, connect.Encode(op, nil)); err != nil {
			t.Fatalf("send %q: %v", op, err)
		}
	}
	for i := 0; i < 3; i++ { // the stream runs on after both frames
		if _, _, err := c.ReadMessage(); err != nil {
			t.Fatalf("frame %d after PAUSE/RESUME: %v", i, err)
		}
	}
	sl.stall.Store(true)
	for deadline := time.Now().Add(3 * time.Second); sl.blocked.Load() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the output pump never reached a stalled write")
		}
	}
	time.Sleep(300 * time.Millisecond)
	if n := sl.blocked.Load(); n != 1 {
		t.Errorf("%d writes were attempted behind a client that is not reading, want 1: output is being queued", n)
	}
}
