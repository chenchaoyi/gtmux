//go:build !darwin && !linux

package codexrpc

import (
	"context"
	"errors"
	"net"

	"github.com/gorilla/websocket"
)

func connectLocal(context.Context) (connection, error) {
	return connection{}, errors.New("local Codex control socket unavailable on this platform")
}

func DialLocal(context.Context) (*websocket.Conn, error) {
	return nil, errors.New("local Codex control socket unavailable on this platform")
}

func LocalPeerPID(net.Conn) (int, bool) { return 0, false }
