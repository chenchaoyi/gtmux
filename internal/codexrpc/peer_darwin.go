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

// LocalPeerPID authenticates the private Unix peer without trusting a supplied
// PID field or inherited environment. A missing credential fails closed.
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
		cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		valid = err == nil && cred.Uid == uint32(os.Geteuid())
		pid, err = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		valid = valid && err == nil && pid > 1
	})
	return pid, err == nil && valid
}
