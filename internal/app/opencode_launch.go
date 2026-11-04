package app

import (
	"context"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func opencodeStandaloneArgs(args []string, version string) []string {
	for _, a := range args {
		name, _, _ := strings.Cut(a, "=")
		switch name {
		case "--standalone", "--no-standalone", "--server", "--help", "-h", "--version":
			return args
		}
	}
	if !agents.OpenCodeInteractiveArgs(args) {
		return args
	}
	major, _ := strconv.Atoi(strings.Split(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(version), "opencode "), "v"), ".")[0])
	if major != 2 {
		return args
	}
	return append([]string{"--standalone"}, args...)
}

// gtmux-created V2 terminal sessions use a leased private server. The CLI owns
// its lifetime and stdin; gtmux does not start/reconfigure the shared service.
func cmdOpencode(args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	bin, err := exec.LookPath("opencode")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gtmux: OpenCode executable unavailable")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	probe := exec.CommandContext(ctx, bin, "--version")
	probe.WaitDelay = time.Second
	version, _ := probe.Output()
	cancel()
	args = opencodeStandaloneArgs(args, string(version))
	diag.Did("act.opencode.launch", "", diag.OK, "handing terminal to native OpenCode")
	if err = syscall.Exec(bin, append([]string{bin}, args...), os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "gtmux:", err)
	}
	return 1
}
