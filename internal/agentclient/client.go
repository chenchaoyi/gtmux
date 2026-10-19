// Package agentclient verifies terminal attachments independently of harness
// identity. Client adapters never turn saved history into an input capability.
package agentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const ProtocolVersion = 1

var panePattern = regexp.MustCompile(`^%[0-9]+$`)
var instancePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Attachment is a short-lived native bridge witness, never a caller-supplied
// path. Session and home identity are checked against the configured harness.
type Attachment struct {
	Version       int    `json:"version"`
	Agent         string `json:"agent"`
	Client        string `json:"client"`
	Pane          string `json:"pane"`
	PID           int    `json:"pid"`
	Instance      string `json:"instance"`
	HomeID        string `json:"home_id"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	ServerEpoch   string `json:"server_epoch"`
	BindingEpoch  int64  `json:"binding_epoch"`
	StartedAt     int64  `json:"started_at"`
	UpdatedAt     int64  `json:"updated_at"`
	InputReady    bool   `json:"input_ready"`
	NativeSession bool   `json:"native_session"`
}

func Dir() string { return filepath.Join(state.Dir(), "terminal-clients") }

// Inspector supplies fresh process and tmux facts. Tests exercise the same
// ownership rules with synthetic processes, without inspecting a real session.
type Inspector struct {
	Now     time.Time
	Process func(int) (int, string, error)
	Pane    func(string) (int, string, error)
	Session func(string, string, string) bool
}

type adapter struct {
	process   func(string) bool
	current   func(Attachment) bool
	lifecycle bool // native start/wait/end events are observed by the admitted bridge
}

var adapters = map[string]adapter{
	"dst": {process: dstProcess, current: dstCurrent, lifecycle: true},
}

// ProvidesLifecycle is an adapter capability, not verification of an attachment.
// Callers must first obtain a current attachment through Load/All.
func ProvidesLifecycle(client string) bool { return adapters[client].lifecycle }

func declared(agent, client string) bool {
	m, ok := agents.For(agent)
	if !ok {
		return false
	}
	for _, key := range m.TerminalClients {
		if key == client {
			return true
		}
	}
	return false
}

func productionInspector() Inspector {
	return Inspector{time.Now(), processInfo, func(pane string) (int, string, error) {
		parts := strings.Split(strings.TrimSpace(tmux.Display(pane, "#{pane_pid}\t#{start_time}")), "\t")
		if len(parts) != 2 {
			return 0, "", errors.New("terminal pane is unavailable")
		}
		pid, err := strconv.Atoi(parts[0])
		return pid, parts[1], err
	}, func(agent, home, id string) bool {
		if agent != "dsh" || home != dsh.HomeID() {
			return false
		}
		s, err := dsh.Sessions.Resolve(id)
		return err == nil && s.ID == id && s.Origin != "subagent" || os.IsNotExist(err)
	}}
}

// Load rejects stale, conflicting and unverified attachments. A current bridge
// must still agree with its client's own foreground discovery and live ancestry.
func Load(dir string, inspect Inspector) []Attachment {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	entries, err := f.ReadDir(257)
	_ = f.Close()
	if err != nil && err != io.EOF || len(entries) > 256 {
		return nil
	}
	var candidates []Attachment
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var a Attachment
		if readOwnedJSON(filepath.Join(dir, entry.Name()), &a, 16<<10) != nil {
			continue
		}
		provider, ok := adapters[a.Client]
		if !ok || !declared(a.Agent, a.Client) || a.Version != ProtocolVersion ||
			!panePattern.MatchString(a.Pane) || !instancePattern.MatchString(a.Instance) ||
			!a.NativeSession || a.PID <= 0 || a.SessionID == "" || len(a.SessionID) > 256 ||
			a.BindingEpoch <= 0 || a.StartedAt <= 0 || a.ServerEpoch == "" {
			continue
		}
		age := inspect.Now.UnixMilli() - a.UpdatedAt
		if age < -1000 || age > 5000 || a.StartedAt > a.UpdatedAt || !inspect.Session(a.Agent, a.HomeID, a.SessionID) {
			continue
		}
		panePID, epoch, err := inspect.Pane(a.Pane)
		if err != nil || panePID <= 0 || epoch != a.ServerEpoch {
			continue
		}
		parent, command, err := inspect.Process(a.PID)
		if err != nil || !provider.process(command) {
			continue
		}
		if a.PID == panePID {
			parent = panePID
		}
		seen := map[int]bool{a.PID: true}
		for depth := 0; parent != panePID && parent > 1 && depth < 32; depth++ {
			if seen[parent] {
				parent = 0
				break
			}
			seen[parent] = true
			parent, _, err = inspect.Process(parent)
			if err != nil {
				break
			}
		}
		if err != nil || parent != panePID || !provider.current(a) {
			continue
		}
		candidates = append(candidates, a)
	}
	panes, sessions := map[string]int{}, map[string]int{}
	for _, a := range candidates {
		panes[a.Pane]++
		sessions[a.Agent+"\x00"+a.HomeID+"\x00"+a.SessionID]++
	}
	var out []Attachment
	for _, a := range candidates {
		if panes[a.Pane] == 1 && sessions[a.Agent+"\x00"+a.HomeID+"\x00"+a.SessionID] == 1 {
			out = append(out, a)
		}
	}
	return out
}

func All() []Attachment { return Load(Dir(), productionInspector()) }

func ForPane(pane string) (Attachment, bool) {
	for _, a := range All() {
		if a.Pane == pane {
			return a, true
		}
	}
	return Attachment{}, false
}

func ForSession(agent, home, id string) (Attachment, bool) {
	for _, a := range All() {
		if a.Agent == agent && a.HomeID == home && a.SessionID == id {
			return a, true
		}
	}
	return Attachment{}, false
}

func Same(a, b Attachment) bool {
	return a.Agent == b.Agent && a.Client == b.Client && a.HomeID == b.HomeID &&
		a.SessionID == b.SessionID && a.Pane == b.Pane && a.PID == b.PID &&
		a.Instance == b.Instance && a.ServerEpoch == b.ServerEpoch && a.BindingEpoch == b.BindingEpoch
}

func readOwnedJSON(path string, value any, limit int64) error {
	return ReadOwnedJSON(path, value, limit)
}

// ReadOwnedJSON reads bounded, current-user, regular metadata without following file links.
func ReadOwnedJSON(path string, value any, limit int64) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Size() > limit || fi.Mode().Perm()&0022 != 0 {
		return errors.New("untrusted terminal metadata")
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return errors.New("foreign terminal metadata")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(fi, opened) {
		return errors.New("terminal metadata changed")
	}
	decoder := json.NewDecoder(io.LimitReader(f, limit+1))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("unexpected terminal metadata")
	}
	return nil
}

func processInfo(pid int) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,command=")
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil || len(out) > 32768 {
		return 0, "", fmt.Errorf("terminal process is unavailable")
	}
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return 0, "", errors.New("invalid terminal process")
	}
	parent, err := strconv.Atoi(parts[0])
	return parent, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), parts[0])), err
}
