package codexterminal

import (
	"strconv"
	"strings"
)

func stdinFromLsof(raw string, pid int) string {
	var path string
	fd := ""
	owner := false
	headers := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "p") {
			headers++
			owner = line == "p"+strconv.Itoa(pid)
			fd = ""
			if !owner || headers > 1 {
				return ""
			}
		}
		if strings.HasPrefix(line, "f") {
			fd = strings.TrimPrefix(line, "f")
		}
		if strings.HasPrefix(line, "n") && owner && fd == "0" {
			if path != "" {
				return ""
			}
			path = strings.TrimPrefix(line, "n")
		}
	}
	if !strings.HasPrefix(path, "/dev/") {
		return ""
	}
	return tty(path)
}
