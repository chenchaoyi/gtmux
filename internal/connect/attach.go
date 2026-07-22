package connect

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

// detachKey is a local force-detach: Ctrl-] (0x1D, telnet's escape) — rare in normal
// use. The CLEAN detach is tmux's own `<prefix> d`, which exits the server-side client
// and closes the stream; Ctrl-] is the escape hatch for a stuck link.
const detachKey = 0x1D

// wsURL rewrites an http(s) base into a ws(s) /api/attach URL for a pane id. It carries
// the local terminal's $TERM so the remote tmux client can honor it (the server uses it
// only if the remote has terminfo for it, else a safe fallback).
func wsURL(base, paneID string) string {
	u := base
	if strings.HasPrefix(u, "https://") {
		u = "wss://" + strings.TrimPrefix(u, "https://")
	} else if strings.HasPrefix(u, "http://") {
		u = "ws://" + strings.TrimPrefix(u, "http://")
	}
	return strings.TrimRight(u, "/") + "/api/attach?id=" + url.QueryEscape(paneID) +
		"&term=" + url.QueryEscape(os.Getenv("TERM"))
}

// RunAttach opens the attach WebSocket and passes the local terminal through to the
// remote pane: raw mode, stdin→INPUT (unless readOnly), OUTPUT→stdout, SIGWINCH→RESIZE.
// It ALWAYS restores the terminal on exit (normal detach, error, or signal).
func RunAttach(base, token, paneID string, readOnly bool) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("gtmux attach needs an interactive terminal")
	}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(base, paneID), http.Header{
		"Authorization": {"Bearer " + token},
	})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("attach refused (HTTP %d)", resp.StatusCode)
		}
		return fmt.Errorf("can't reach the server: %w", err)
	}
	defer conn.Close()

	// Raw mode — and restore it no matter how we exit.
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	restore := func() { _ = term.Restore(fd, oldState) }
	defer restore()

	// Tell the server our current size, then keep it in step on SIGWINCH.
	sendSize(conn)
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			sendSize(conn)
		}
	}()

	done := make(chan error, 2)
	// The server's authoritative tmux cursor (OpCursor). Stage 1 of
	// attach-predictive-echo only TRACKS it — nothing predicts yet; stage 2 draws at it
	// and reconciles against it.
	var cursor cursorTracker

	// OUTPUT → stdout (raw). Ends when the pane detaches/exits (WS close).
	go func() {
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				done <- nil // clean end (server closed / detached)
				return
			}
			if mt != websocket.BinaryMessage {
				continue
			}
			op, payload, ok := Decode(data)
			if !ok {
				continue
			}
			switch op {
			case OpOutput:
				if _, werr := os.Stdout.Write(payload); werr != nil {
					done <- werr
					return
				}
			case OpCursor:
				// attach-predictive-echo stage 1: track the AUTHORITATIVE tmux cursor the
				// server streams, so a later stage can predict at it and reconcile against
				// it. Display is untouched here — this only records state.
				if c, cok := DecodeCursor(payload); cok {
					cursor.set(c)
				}
			}
		}
	}()

	// stdin → INPUT. A read-only attach never sends. Ctrl-] force-detaches locally.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if i := indexByte(buf[:n], detachKey); i >= 0 {
					done <- nil // local detach
					return
				}
				if !readOnly {
					if werr := conn.WriteMessage(websocket.BinaryMessage, Encode(OpInput, buf[:n])); werr != nil {
						done <- werr
						return
					}
				}
			}
			if err != nil {
				if err != io.EOF {
					done <- err
				} else {
					done <- nil
				}
				return
			}
		}
	}()

	err = <-done
	restore()
	// Best-effort clean close so the server tears the tmux client down promptly.
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	return err
}

// sendSize sends the current terminal size as a RESIZE frame (best-effort).
func sendSize(conn *websocket.Conn) {
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		_ = conn.WriteMessage(websocket.BinaryMessage, EncodeResize(w, h))
	}
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// cursorTracker holds the last cursor the server reported (OpCursor). Written by the
// output reader, read by the (future) predictor on the input goroutine — hence the mutex.
type cursorTracker struct {
	mu   sync.Mutex
	c    Cursor
	have bool
}

func (t *cursorTracker) set(c Cursor) {
	t.mu.Lock()
	t.c, t.have = c, true
	t.mu.Unlock()
}

// NOTE: the reader for this state (a `get`) lands in stage 2 with the predictor that
// consumes it — `have` stays false against an older server that never sends OpCursor,
// which is exactly the condition under which prediction must stay off.
