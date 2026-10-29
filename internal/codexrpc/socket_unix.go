//go:build darwin || linux

package codexrpc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/gorilla/websocket"
)

// localSocket implements Codex's verified_local_daemon_socket rendezvous check.
// It only reads existing paths and never starts, repairs or reconfigures a server.
func localSocket() (string, error) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = filepath.Join(state.Home(), ".codex")
	}
	parent, err := filepath.EvalSymlinks(filepath.Join(home, "app-server-control"))
	if err != nil {
		return "", err
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	rendezvous := filepath.Join(parent, "app-server-control.sock")
	tmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(tmp, "codex-daemon-"+strconv.Itoa(os.Geteuid()))
	i, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !i.IsDir() || i.Mode().Perm() != 0700 || i.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return "", fmt.Errorf("unverified local socket directory")
	}
	path := filepath.Join(dir, fmt.Sprintf("%x", sha256.Sum256([]byte(rendezvous))))
	link, err := os.Readlink(rendezvous)
	if err != nil || link != path {
		return "", fmt.Errorf("unverified local socket rendezvous")
	}
	i, err = os.Lstat(path)
	if err != nil {
		return "", err
	}
	if i.Mode()&os.ModeSocket == 0 || i.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return "", fmt.Errorf("unverified local socket endpoint")
	}
	return path, nil
}

func connectLocal(ctx context.Context) (connection, error) {
	ws, err := DialLocal(ctx)
	if err != nil {
		return connection{}, err
	}
	ws.SetReadLimit(2 << 20)
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			ws.Close()
		case <-finished:
		}
	}()
	stream := &jsonFrames{ws: ws}
	pid, _ := LocalPeerPID(ws.UnderlyingConn())
	process, _ := launchhost.ReadAgent(pid)
	return connection{peerPID: pid, peerStart: process.Start, reader: stream, writer: stream, close: func() { close(finished); ws.Close() }}, nil
}

// DialLocal opens only the already-running, verified user-local daemon. It does
// not read history, start a server, resume a thread or enable remote control.
func DialLocal(ctx context.Context) (*websocket.Conn, error) {
	path, err := localSocket()
	if err != nil {
		return nil, err
	}
	d := websocket.Dialer{
		HandshakeTimeout: 8 * time.Second,
		NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
			if err != nil {
				return nil, err
			}
			fresh, checkErr := localSocket()
			if checkErr != nil || fresh != path || !sameUserPeer(raw) {
				raw.Close()
				return nil, fmt.Errorf("local socket changed or peer unverified")
			}
			if deadline, ok := ctx.Deadline(); ok {
				_ = raw.SetDeadline(deadline)
			}
			return raw, nil
		},
	}
	// The address is a WebSocket Host header only; NetDialContext cannot use TCP.
	ws, response, err := d.DialContext(ctx, "ws://codex.local/", nil)
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		return nil, err
	}
	ws.SetReadLimit(64 << 20)
	return ws, nil
}

// A JSON encoder write is one complete frame; reads preserve frame boundaries
// without treating the private control socket as a newline-based stdio service.
type jsonFrames struct {
	ws     *websocket.Conn
	reader io.Reader
}

func (f *jsonFrames) Write(p []byte) (int, error) {
	if err := f.ws.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (f *jsonFrames) Read(p []byte) (int, error) {
	for {
		if f.reader == nil {
			kind, r, err := f.ws.NextReader()
			if err != nil {
				return 0, err
			}
			if kind != websocket.TextMessage {
				return 0, fmt.Errorf("unexpected binary control message")
			}
			f.reader = r
		}
		n, err := f.reader.Read(p)
		if err == io.EOF {
			f.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}
