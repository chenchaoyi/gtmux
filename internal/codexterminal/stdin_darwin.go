package codexterminal

import (
	"context"
	"strconv"
)

func stdinTTY(ctx context.Context, pid int) string {
	raw := run(ctx, "/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-d", "0", "-Fn")
	return stdinFromLsof(raw, pid)
}
