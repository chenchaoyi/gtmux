package codexrpc

import (
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func sameUserPeer(c net.Conn) bool {
	_, ok := LocalPeerPID(c)
	return ok
}

func LocalPeerPID(c net.Conn) (int, bool) {
	u, ok := c.(*net.UnixConn)
	if !ok {
		return 0, false
	}
	raw, err := u.SyscallConn()
	if err != nil {
		return 0, false
	}
	var valid bool
	var pid int
	err = raw.Control(func(fd uintptr) {
		cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		valid = err == nil && cred.Uid == uint32(os.Geteuid())
		if valid {
			pid = int(cred.Pid)
		}
	})
	return pid, err == nil && valid && pid > 1
}
