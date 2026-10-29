package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexbridge"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/toolpath"
	"golang.org/x/term"
)

func nativeCodexBinary() string {
	shim := filepath.Join(state.Dir(), "bin", "codex")
	for _, dir := range append(filepath.SplitList(os.Getenv("PATH")), toolpath.SearchDirs()...) {
		candidate := filepath.Join(dir, "codex")
		absolute, err := filepath.Abs(candidate)
		if err == nil && absolute != shim && toolpath.IsExecutable(absolute) {
			return absolute
		}
	}
	return ""
}

// terminalPane derives physical launch identity from the controlling terminal,
// rather than the shared backend's inherited TMUX_PANE.
func terminalPane() string {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return ""
	}
	device, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	found := ""
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_tty}") {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			continue
		}
		p, err := os.Stat(fields[1])
		if err != nil || p.Mode()&os.ModeCharDevice == 0 {
			continue
		}
		terminal, ok := p.Sys().(*syscall.Stat_t)
		if !ok || terminal.Rdev != device.Rdev {
			continue
		}
		if found != "" {
			return ""
		}
		found = fields[0]
	}
	return found
}

func cmdCodex(args []string) int {
	bin := nativeCodexBinary()
	if bin == "" {
		fmt.Fprintln(os.Stderr, "gtmux: Codex executable unavailable")
		return 1
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if term.IsTerminal(0) && term.IsTerminal(1) && codexbridge.Eligible(args) && os.Getenv("CODEX_EXEC_SERVER_URL") == "" {
		if pane := terminalPane(); pane != "" && supportsCodexBridge(bin, args) && ensureCodexDaemon(bin) {
			if code := codexbridge.Run(bin, args, pane); code >= 0 {
				return code
			}
		}
	}
	// Exact argv, stdin, environment and exit behavior remain native for older
	// clients and excluded launches. Never retry after any new client has started.
	diag.Did("act.codex.launch", "", diag.OK, "handing terminal to native Codex", "mode", "native-handoff")
	if err := syscall.Exec(bin, append([]string{bin}, args...), os.Environ()); err != nil {
		diag.Did("act.codex.launch", "", diag.Failed, "native Codex handoff failed", "error", err)
		fmt.Fprintln(os.Stderr, "gtmux:", err)
	}
	return 1
}

func supportsCodexBridge(bin string, args []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--help")
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	outPipe, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return false
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = outPipe.Close()
		case <-done:
		}
	}()
	out, err := io.ReadAll(io.LimitReader(outPipe, (128<<10)+1))
	close(done)
	if len(out) > 128<<10 {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	return err == nil && waitErr == nil && len(out) <= 128<<10 && strings.Contains(string(out), "--remote") && strings.Contains(string(out), "--no-daemon") && codexbridge.HelpAdmits(args, string(out))
}

func existingCodexDaemon() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, err := codexrpc.DialLocal(ctx)
	if err != nil {
		return false
	}
	connection.Close()
	return true
}

var codexDaemonProbe = existingCodexDaemon

func ensureCodexDaemon(bin string) bool {
	if codexDaemonProbe() {
		return true
	}
	alias := filepath.Join(codexHome(), "app-server-control", "app-server-control.sock")
	if info, err := os.Lstat(alias); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(alias))
		if err != nil {
			return false
		}
		parent, err = filepath.Abs(parent)
		if err != nil {
			return false
		}
		tmp, err := filepath.EvalSymlinks("/tmp")
		if err != nil {
			return false
		}
		physical := filepath.Join(tmp, "codex-daemon-"+strconv.Itoa(os.Geteuid()), fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Join(parent, filepath.Base(alias))))))
		if link, err := os.Readlink(alias); err != nil || link != physical {
			return false
		}
	} else if !os.IsNotExist(err) {
		return false
	}
	// This is an explicit interactive launch, not list/history discovery. The
	// official start operation leaves an already-running daemon and its access
	// settings intact; restart/update/bootstrap/remote-control are never used.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "app-server", "daemon", "start")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil && codexDaemonProbe()
}
