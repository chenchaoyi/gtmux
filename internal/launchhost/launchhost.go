// Package launchhost observes historical hosting context, never control authority.
package launchhost

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Display is the only projection exposed to clients. Paths and process IDs stay private.
type Display struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ObservedAt int64  `json:"observed_at"`
}

// Evidence is a comparable snapshot so native generation comparisons remain exact.
// These identities describe capture time; they must not authorize liveness or actions.
type Evidence struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ObservedAt int64  `json:"observed_at"`
	AgentPID   int    `json:"agent_pid"`
	AgentStart string `json:"agent_start"`
	HostPID    int    `json:"host_pid"`
	HostStart  string `json:"host_start"`
}

func (e Evidence) Display() *Display {
	if !validName(e.Name, e.Kind) || e.ObservedAt <= 0 {
		return nil
	}
	return &Display{e.Name, e.Kind, e.ObservedAt}
}

const budget = 350 * time.Millisecond
const maxDepth = 8

type process struct {
	PID, Parent    int
	Start, Command string
}
type inspector struct {
	read func(int) (process, bool)
	app  func(string) string
}

// Observe reads only this hook's ancestors, with a shared deadline and output cap.
func Observe(commands []string) Evidence {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	i := inspector{read: func(pid int) (process, bool) { return readProcess(ctx, pid) }, app: func(path string) string { return appName(ctx, path) }}
	return i.observe(os.Getppid(), commands, time.Now().Unix())
}
func (i inspector) observe(pid int, commands []string, now int64) Evidence {
	var chain []process
	var agent process
	seen := map[int]bool{}
	for range maxDepth {
		if pid <= 1 || seen[pid] {
			return Evidence{}
		}
		seen[pid] = true
		p, ok := i.read(pid)
		if !ok || p.PID != pid || p.Start == "" || p.Parent == pid {
			return Evidence{}
		}
		chain = append(chain, p)
		pid = p.Parent
		name := strings.TrimPrefix(filepath.Base(p.Command), "-")
		isAgent := false
		for _, c := range commands {
			if name == c {
				isAgent = true
				break
			}
		}
		if agent.PID == 0 {
			if isAgent {
				agent = p
				continue
			}
			if shell(name) {
				continue
			}
			return Evidence{}
		}
		if isAgent || shell(name) {
			continue
		}
		if boundary(name) {
			return Evidence{}
		}
		host, kind := i.app(p.Command), "app"
		if host == "" {
			if interpreter.MatchString(name) {
				return Evidence{}
			}
			host, kind = name, "process"
		}
		if !validName(host, kind) {
			return Evidence{}
		}
		// Reject a chain that changed while we were sampling (including PID reuse).
		for _, old := range chain {
			fresh, ok := i.read(old.PID)
			if !ok || fresh != old {
				return Evidence{}
			}
		}
		return Evidence{host, kind, now, agent.PID, agent.Start, p.PID, p.Start}
	}
	return Evidence{}
}
func shell(s string) bool {
	switch s {
	case "sh", "bash", "zsh", "dash", "fish", "ksh", "gtmux", "env", "nohup", "timeout":
		return true
	}
	return false
}
func boundary(s string) bool {
	switch s {
	case "tmux", "launchd", "systemd", "init", "sshd", "login":
		return true
	}
	return false
}

var interpreter = regexp.MustCompile(`^(node(js)?|bun|deno|python[0-9.]*|ruby[0-9.]*|perl[0-9.]*|java|dotnet|php[0-9.]*)$`)

func validName(s, kind string) bool {
	if s == "" || strings.TrimSpace(s) != s || len([]rune(s)) > 80 || (kind != "app" && kind != "process") {
		return false
	}
	letter := false
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || r == '/' || r == '\\' {
			return false
		}
		if unicode.IsLetter(r) {
			letter = true
		}
		if kind == "process" && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("._+-", r) {
			return false
		}
	}
	return kind == "app" || letter
}

// Encode/Decode hand off fresh display evidence to a detached worker, not a credential.
func Encode(e Evidence) string {
	if e.Display() == nil {
		return ""
	}
	b, _ := json.Marshal(e)
	return string(b)
}
func Decode(raw string, now int64) Evidence {
	var e Evidence
	if len(raw) > 1024 || json.Unmarshal([]byte(raw), &e) != nil || e.Display() == nil || e.AgentPID <= 1 || e.HostPID <= 1 || e.AgentStart == "" || e.HostStart == "" || e.ObservedAt > now+2 || now-e.ObservedAt > 60 {
		return Evidence{}
	}
	return e
}

type boundedOutput struct {
	strings.Builder
	large bool
}

func (w *boundedOutput) Write(b []byte) (int, error) {
	n := len(b)
	if w.Len()+n > 4096 {
		w.large = true
		return n, nil
	}
	_, _ = w.Builder.Write(b)
	return n, nil
}
func run(ctx context.Context, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 50 * time.Millisecond
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out boundedOutput
	cmd.Stdout = &out
	if cmd.Run() != nil || out.large {
		return ""
	}
	return strings.TrimSpace(out.String())
}

var processLine = regexp.MustCompile(`^([0-9]+)\s+([A-Za-z]{3}\s+[A-Za-z]{3}\s+[0-9]{1,2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2}\s+[0-9]{4})\s+([^\r\n]+)$`)

func readProcess(ctx context.Context, pid int) (process, bool) {
	line := run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,lstart=,comm=")
	m := processLine.FindStringSubmatch(line)
	if m == nil {
		return process{}, false
	}
	parent, err := strconv.Atoi(m[1])
	if err != nil {
		return process{}, false
	}
	birth := strings.Join(strings.Fields(m[2]), " ")
	if _, err := time.Parse("Mon Jan 2 15:04:05 2006", birth); err != nil {
		return process{}, false
	}
	return process{pid, parent, birth, m[3]}, true
}
