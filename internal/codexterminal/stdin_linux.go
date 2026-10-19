package codexterminal

import (
	"context"
	"os"
	"strconv"
	"strings"
)

func stdinTTY(_ context.Context, pid int) string {
	path, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/fd/0")
	if err != nil || !strings.HasPrefix(path, "/dev/") {
		return ""
	}
	return tty(path)
}
