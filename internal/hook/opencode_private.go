package hook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type opencodeProcess struct {
	parent  int
	command string
	args    []string
}

// A V2 service's inherited pane is not ownership. Only the leased private server
// launched by a live --standalone CLI can use its verified ancestor pane.
func opencodePrivateChain(pid int, read func(int) opencodeProcess) bool {
	server := false
	for range 8 {
		if pid <= 1 {
			return false
		}
		p := read(pid)
		if p.parent <= 1 || p.parent == pid {
			return false
		}
		if filepath.Base(p.command) == "opencode" {
			has := func(flag string) bool {
				for _, arg := range p.args {
					if arg == flag {
						return true
					}
				}
				return false
			}
			option := func(flag string) bool {
				for _, arg := range p.args {
					if arg == flag || strings.HasPrefix(arg, flag+"=") {
						return true
					}
				}
				return false
			}
			if !server {
				if !has("serve") || !has("--stdio") || option("--service") {
					return false
				}
				server = true
			} else {
				return (has("--standalone") || has("--standalone=true")) &&
					!has("--standalone=false") && !option("--no-standalone") && !has("serve") && !option("--server")
			}
		} else if server && !isShellComm(filepath.Base(p.command)) {
			return false
		}
		pid = p.parent
	}
	return false
}

func opencodePrivatePane() string {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	read := func(pid int) opencodeProcess {
		out, err := exec.CommandContext(ctx, "ps", "-ww", "-o", "ppid=,args=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return opencodeProcess{}
		}
		fields := strings.Fields(string(out))
		if len(fields) < 2 {
			return opencodeProcess{}
		}
		parent, _ := strconv.Atoi(fields[0])
		return opencodeProcess{parent, fields[1], fields[1:]}
	}
	if !opencodePrivateChain(os.Getppid(), read) {
		return ""
	}
	return paneFromAncestry()
}
