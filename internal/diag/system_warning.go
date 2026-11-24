package diag

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

func writeSystemStoreWarning(message string) bool {
	return sendSystemStoreWarning(message, []string{"/dev/log", "/var/run/syslog", "/var/run/log"})
}

// A local syslog socket avoids the failing log volume and the shared terminal.
// Bound the whole handoff, including a listener that has stopped reading.
func sendSystemStoreWarning(message string, sockets []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	const userWarningPriority = (1 << 3) | 4 // LOG_USER | LOG_WARNING
	packet := []byte(fmt.Sprintf("<%d>%s gtmux[%d]: %s\n", userWarningPriority, time.Now().Format(time.Stamp), os.Getpid(), message))
	var dialer net.Dialer
	for _, network := range []string{"unixgram", "unix"} {
		for _, path := range sockets {
			conn, err := dialer.DialContext(ctx, network, path)
			if err != nil {
				continue
			}
			if conn.SetWriteDeadline(deadline) != nil {
				_ = conn.Close()
				continue
			}
			n, err := conn.Write(packet)
			_ = conn.Close()
			if err == nil && n == len(packet) {
				return true
			}
		}
	}
	return false
}
