// Package codexterminal verifies interactive Codex clients beneath generic launchers.
// A process name or ancestry alone never authorizes terminal input.
package codexterminal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Pane is a fresh tmux observation, not a session binding.
type Pane struct {
	ID, Command, TTY, Cwd, Loc string
	PID                        int
}

// Client identifies the terminal process incarnation. Session identity is checked
// separately by hooks and delivery receipts; no arguments or prompt text are kept.
type Client struct {
	PID                   int
	Start, RootStart, TTY string
}
type process struct {
	pid, parent, group, foreground int
	tty, start, command            string
}
type inspector struct {
	snapshot func() map[int]process
	stdin    func(int) string
	pane     func(Pane) bool
}

// Resolve shares two bounded process snapshots across all candidate panes.
// Unknown, changed, piped, detached and ambiguous clients are omitted.
func Resolve(panes []Pane) map[string]Client {
	if len(panes) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	i := inspector{
		snapshot: func() map[int]process {
			return parseProcesses(run(ctx, "ps", "-axo", "pid=,ppid=,pgid=,tpgid=,tty=,lstart=,comm="))
		},
		stdin: func(pid int) string { return stdinTTY(ctx, pid) },
		pane: func(p Pane) bool {
			raw := run(ctx, tmux.Bin, "-u", "display-message", "-p", "-t", p.ID, "#{pane_id}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_command}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}")
			return paneMatches(p, raw)
		},
	}
	return i.resolve(panes)
}

func paneMatches(p Pane, raw string) bool {
	f := strings.Split(raw, "\t")
	return len(f) == 6 && f[0] == p.ID && f[1] == strconv.Itoa(p.PID) && tty(f[2]) == tty(p.TTY) && f[3] == p.Command && (p.Cwd == "" || f[4] == p.Cwd) && (p.Loc == "" || f[5] == p.Loc)
}

func (i inspector) resolve(panes []Pane) map[string]Client {
	before := i.snapshot()
	type candidate struct {
		pane   Pane
		client process
		chain  []int
	}
	var candidates []candidate
	for _, p := range panes {
		client, chain := findClient(p, before)
		if client.pid > 0 && tty(i.stdin(client.pid)) == tty(p.TTY) {
			candidates = append(candidates, candidate{p, client, chain})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	after := i.snapshot()
	out := map[string]Client{}
	for _, c := range candidates {
		stable := true
		for _, pid := range c.chain {
			if before[pid] != after[pid] {
				stable = false
				break
			}
		}
		if !stable || !i.pane(c.pane) || tty(i.stdin(c.client.pid)) != tty(c.pane.TTY) {
			continue
		}
		// A changed branch must not create a second candidate between snapshots.
		fresh, _ := findClient(c.pane, after)
		if fresh != c.client {
			continue
		}
		out[c.pane.ID] = Client{c.client.pid, c.client.start, before[c.pane.PID].start, tty(c.pane.TTY)}
	}
	return out
}

func findClient(p Pane, procs map[int]process) (process, []int) {
	// A tool child must not turn a different registered harness into a Codex
	// launcher. Claude also reports a version as its foreground command.
	if key, known := agents.CommandKeys()[filepath.Base(p.Command)]; known && key != "codex" {
		return process{}, nil
	}
	if agents.LooksLikeVersion(filepath.Base(p.Command)) {
		return process{}, nil
	}
	root, ok := procs[p.PID]
	wantTTY := tty(p.TTY)
	if !ok || p.ID == "" || p.PID <= 1 || wantTTY == "" || tty(root.tty) != wantTTY || root.foreground <= 0 {
		return process{}, nil
	}
	type step struct {
		pid   int
		chain []int
	}
	queue := []step{{p.PID, nil}}
	seen := map[int]bool{}
	var found process
	var path []int
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur.pid] || len(cur.chain) > 16 || len(seen) >= 256 {
			return process{}, nil
		}
		seen[cur.pid] = true
		proc, ok := procs[cur.pid]
		if !ok {
			continue
		}
		chain := append(append([]int(nil), cur.chain...), cur.pid)
		if filepath.Base(proc.command) == "codex" {
			if proc.start != "" && tty(proc.tty) == wantTTY && proc.group == root.foreground && proc.foreground == root.foreground {
				if found.pid != 0 {
					return process{}, nil
				}
				found, path = proc, chain
			}
			continue // tools under another Codex process are not independent frontends
		}
		for pid, child := range procs {
			if child.parent == cur.pid {
				queue = append(queue, step{pid, chain})
			}
		}
	}
	return found, path
}

func tty(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "/dev/")
	if s == "" || s == "?" || s == "??" || s == "-" {
		return ""
	}
	// stdin must be the pane's character device, not a pipe/socket/file with a
	// coincidentally similar basename. Linux pseudo-terminals are pts/N.
	if !strings.HasPrefix(s, "tty") && !strings.HasPrefix(s, "pts/") {
		return ""
	}
	return s
}

func parseProcesses(raw string) map[int]process {
	out := map[int]process{}
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 11 {
			continue
		}
		pid, e := strconv.Atoi(f[0])
		if e != nil || pid <= 1 {
			continue
		}
		parent, e := strconv.Atoi(f[1])
		if e != nil || parent < 0 {
			continue
		}
		group, e := strconv.Atoi(f[2])
		if e != nil {
			continue
		}
		foreground, e := strconv.Atoi(f[3])
		if e != nil {
			continue
		}
		start := strings.Join(f[5:10], " ")
		if _, e := time.Parse("Mon Jan 2 15:04:05 2006", start); e != nil {
			continue
		}
		if _, exists := out[pid]; exists {
			return nil
		}
		out[pid] = process{pid, parent, group, foreground, tty(f[4]), start, strings.Join(f[10:], " ")}
	}
	return out
}

type limitedOutput struct {
	data  []byte
	large bool
}

func (w *limitedOutput) Write(p []byte) (int, error) {
	if len(w.data)+len(p) > 2<<20 {
		w.large = true
		return len(p), nil
	}
	w.data = append(w.data, p...)
	return len(p), nil
}

// Abandon a kernel-stalled reader at the shared deadline, as radar does. The
// discarded output is never read concurrently with the subprocess writer.
func run(ctx context.Context, name string, args ...string) string {
	if name == "" {
		return ""
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 50 * time.Millisecond
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out limitedOutput
	cmd.Stdout = &out
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil || out.large {
			return ""
		}
		return strings.TrimSpace(string(out.data))
	case <-ctx.Done():
		return ""
	}
}
