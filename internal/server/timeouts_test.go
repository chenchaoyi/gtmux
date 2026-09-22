package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The serve used http.ListenAndServe, which bounds nothing: a client could take forever
// to send its request headers and hold a connection open the whole time, and on the local
// network nothing sits in front of the serve to cut that off (2026-09-22 self-check).
//
// What must be bounded is the HEADER, not the response. The event stream and the attach
// terminal keep one response open for as long as someone is reading, so these pin both
// halves: a stalled header is cut off, and a long response is not.

// runServer serves the serve's own http.Server on a free port, with the timeouts shrunk
// so the test takes a moment, not ten seconds.
func runServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	oldHeader, oldIdle := readHeaderTimeout, idleTimeout
	readHeaderTimeout, idleTimeout = 300*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { readHeaderTimeout, idleTimeout = oldHeader, oldIdle })

	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{})
	srv := s.httpServer()
	if handler != nil {
		srv.Handler = handler
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return l.Addr().String()
}

func TestAConnectionThatNeverFinishesItsHeadersIsClosed(t *testing.T) {
	addr := runServer(t, nil)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Start a request and never finish it.
	if _, err := io.WriteString(c, "GET /api/health HTTP/1.1\r\nHost: x\r\n"); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	_, err = c.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("the server answered a request whose headers never ended")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("the connection was still open after %v: a stalled header is held forever", time.Since(start).Round(time.Millisecond))
	}
}

func TestAnOrdinaryRequestIsUnaffected(t *testing.T) {
	addr := runServer(t, nil)
	res, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("health answered %d", res.StatusCode)
	}
}

// A response that lasts many times the timeouts must survive: this is the shape of the
// event stream and of an attach session.
func TestALongResponseIsNotCutOff(t *testing.T) {
	const beats = 6 // 6 × 150ms = three times both timeouts
	addr := runServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < beats; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(150 * time.Millisecond)
		}
	}))
	res, err := http.Get("http://" + addr + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got := 0
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data: ") {
			got++
		}
	}
	if got != beats {
		t.Errorf("a stream lasting past the timeouts delivered %d of %d events", got, beats)
	}
}
