package app

import (
	"context"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	chshare "github.com/jpillora/chisel/share"
	"github.com/jpillora/chisel/share/cnet"
	"golang.org/x/crypto/ssh"
)

// directAccountRefused reports whether the Direct server REFUSES this account: an
// explicit authentication failure, never a network one.
//
// Direct accounts are per device now (openspec/changes/direct-per-device-accounts), so
// an account can stop being accepted: its code was revoked, or it was replaced. chisel's
// client meets that as one more connection error and retries it forever, logging
// "Authentication failed" to a stderr gtmux keeps quiet, so the Mac would look like it was
// still connecting, indefinitely, with nothing to say why. Its own Wait reports only
// "connection attempts exhausted", the same words a dead network produces.
//
// So this does what chisel's connectionOnce does, up to the SSH handshake and no further:
// the same WebSocket, subprotocol and client version, then the password. Anything short
// of a refusal (unreachable, timed out, TLS trouble) answers false and is left to the
// long-running client, which is built to wait those out.
func directAccountRefused(ctx context.Context, server, secret string) bool {
	user, pass, ok := strings.Cut(secret, ":")
	if !ok || user == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u := strings.Replace(strings.Replace(server, "https://", "wss://", 1), "http://", "ws://", 1)
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Subprotocols: []string{chshare.ProtocolVersion}}
	ws, _, err := d.DialContext(ctx, u, nil)
	if err != nil {
		return false
	}
	defer ws.Close()
	conn, _, _, err := ssh.NewClientConn(cnet.NewWebSocketConn(ws), "", &ssh.ClientConfig{
		User:          user,
		Auth:          []ssh.AuthMethod{ssh.Password(pass)},
		ClientVersion: "SSH-" + chshare.ProtocolVersion + "-client",
		// The same trust the tunnel client itself extends: gtmux has never pinned the
		// Direct server's host key, and this sends nothing the real connection would not.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // mirrors the client
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return strings.Contains(err.Error(), "unable to authenticate")
	}
	_ = conn.Close()
	return false
}
