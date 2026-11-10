package codexterminal

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Observation describes evidence, not authority. Only an empty Code is verified.
// Ancestors is the rechecked path from the pane root to the first Codex client.
type Observation struct {
	Client
	Ancestors []int
	Code      string
}

var launchRead = run
var launchStdin = stdinTTY

func (o Observation) Retryable() bool {
	return o.Code == "snapshot-unavailable" || o.Code == "process-unavailable" || o.Code == "stdin-unavailable"
}

func (i inspector) observe(panes []Pane) map[string]Observation {
	out := make(map[string]Observation, len(panes))
	before := i.snapshot()
	type candidate struct {
		pane   Pane
		client process
		chain  []int
	}
	var candidates []candidate
	for _, p := range panes {
		if len(before) == 0 {
			out[p.ID] = Observation{Code: "snapshot-unavailable"}
			continue
		}
		c, chain, code := inspectClient(p, before)
		if code != "" {
			out[p.ID] = Observation{Code: code}
			continue
		}
		input := tty(i.stdin(c.pid))
		if input == "" {
			out[p.ID] = Observation{Code: "stdin-unavailable"}
		} else if input != tty(p.TTY) {
			out[p.ID] = Observation{Code: "terminal-mismatch"}
		} else {
			candidates = append(candidates, candidate{p, c, chain})
		}
	}
	if len(candidates) == 0 {
		return out
	}
	after := i.snapshot()
	for _, c := range candidates {
		code := ""
		if len(after) == 0 {
			code = "snapshot-unavailable"
		} else {
			for _, pid := range c.chain {
				if _, exists := after[pid]; !exists {
					code = "process-unavailable"
					break
				}
				if before[pid] != after[pid] {
					code = "process-changed"
					break
				}
			}
		}
		if code == "" && !i.pane(c.pane) {
			code = "pane-changed"
		}
		if code == "" {
			input := tty(i.stdin(c.client.pid))
			if input == "" {
				code = "stdin-unavailable"
			} else if input != tty(c.pane.TTY) {
				code = "terminal-mismatch"
			}
		}
		if code == "" {
			fresh, _, reason := inspectClient(c.pane, after)
			if reason != "" {
				code = reason
			} else if fresh != c.client {
				code = "client-changed"
			}
		}
		result := Observation{Code: code}
		if code == "" {
			result.Client = Client{c.client.pid, c.client.start, before[c.pane.PID].start, tty(c.pane.TTY)}
			result.Ancestors = c.chain
		}
		out[c.pane.ID] = result
	}
	return out
}

// ObserveLaunch uses the same two full-tree snapshots as legacy resolution.
// A known child PID proves launch ancestry, not absence of another foreground
// client. Missing rows remain unavailable evidence, never authority.
func ObserveLaunch(parent context.Context, p Pane, childPID, bridgePID, peerPID int) Observation {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	missingPeer := false
	snapshot := func() map[int]process {
		procs := parseProcesses(launchRead(ctx, "ps", "-axo", "pid=,ppid=,pgid=,tpgid=,tty=,lstart=,comm="))
		if len(procs) > 0 {
			if _, exists := procs[peerPID]; !exists {
				missingPeer = true
			}
		}
		return procs
	}
	i := inspector{snapshot: snapshot, stdin: func(pid int) string { return launchStdin(ctx, pid) }, pane: func(p Pane) bool {
		return paneMatches(p, launchRead(ctx, tmux.Bin, "-u", "display-message", "-p", "-t", p.ID,
			"#{pane_id}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_command}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}"))
	}}
	o := i.observe([]Pane{p})[p.ID]
	if missingPeer && (o.Code == "" || o.Code == "client-unverified" || o.Code == "client-changed" || o.Code == "process-unavailable") {
		o = Observation{Code: "process-unavailable"}
	}
	if ctx.Err() != nil {
		return Observation{Code: "snapshot-unavailable"}
	}
	return o
}

// ReadLaunchPane reads only the requested current pane, within the shared budget.
func ReadLaunchPane(ctx context.Context, pane string) (Pane, bool) {
	raw := launchRead(ctx, tmux.Bin, "-u", "display-message", "-p", "-t", pane,
		"#{pane_id}\t#{pane_current_command}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}")
	f := strings.Split(raw, "\t")
	if len(f) != 6 || f[0] != pane {
		return Pane{}, false
	}
	pid, err := strconv.Atoi(f[2])
	return Pane{ID: pane, Command: f[1], PID: pid, TTY: f[3], Cwd: f[4], Loc: f[5]}, err == nil && pid > 1
}
