package launchhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgentProcess is private hook-to-worker process evidence, separate from the
// display-only launch host. Backend liveness never establishes frontend control.
type AgentProcess struct {
	PID        int    `json:"pid"`
	Comm       string `json:"comm"`
	Start      string `json:"start"`
	ObservedAt int64  `json:"observed_at"`
}

// ObserveAgent captures the nearest recognized agent before a hook is detached.
// It uses the same metadata reader, bounds and chain revalidation as host capture,
// but does not require a recognizable host application.
func ObserveAgent(commands []string) AgentProcess {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	i := inspector{read: func(pid int) (process, bool) { return readProcess(ctx, pid) }}
	return i.agent(os.Getppid(), commands, time.Now().Unix())
}

func (i inspector) agent(pid int, commands []string, now int64) AgentProcess {
	var chain []process
	seen := map[int]bool{}
	for range maxDepth {
		if pid <= 1 || seen[pid] {
			return AgentProcess{}
		}
		seen[pid] = true
		p, ok := i.read(pid)
		if !ok || p.PID != pid || p.Start == "" || p.Parent == pid {
			return AgentProcess{}
		}
		chain = append(chain, p)
		name := strings.TrimPrefix(filepath.Base(p.Command), "-")
		recognized := false
		for _, c := range commands {
			if name == c {
				recognized = true
				break
			}
		}
		if recognized {
			for _, old := range chain {
				fresh, ok := i.read(old.PID)
				if !ok || fresh != old {
					return AgentProcess{}
				}
			}
			return AgentProcess{p.PID, name, p.Start, now}
		}
		if !shell(name) {
			return AgentProcess{}
		}
		pid = p.Parent
	}
	return AgentProcess{}
}

// ReadAgent reads one current process incarnation, without arguments or a scan.
func ReadAgent(pid int) (AgentProcess, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	p, ok := readProcess(ctx, pid)
	if !ok {
		return AgentProcess{}, false
	}
	return AgentProcess{p.PID, filepath.Base(p.Command), p.Start, time.Now().Unix()}, true
}

func EncodeAgent(p AgentProcess) string {
	if !validAgent(p) {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}
func DecodeAgent(raw string, now int64) AgentProcess {
	var p AgentProcess
	if len(raw) > 512 || json.Unmarshal([]byte(raw), &p) != nil || !validAgent(p) || p.ObservedAt > now+2 || now-p.ObservedAt > 60 {
		return AgentProcess{}
	}
	return p
}
func validAgent(p AgentProcess) bool {
	return p.PID > 1 && validName(p.Comm, "process") && p.ObservedAt > 0 && len(p.Start) > 0 && len(p.Start) < 64 && !strings.ContainsAny(p.Start, "\r\n\x00")
}
