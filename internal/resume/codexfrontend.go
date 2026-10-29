package resume

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// CodexFrontend is private evidence from one live, kernel-verified frontend
// connection. A backend PID, inherited environment or saved locator is not this
// evidence. The resume record remains history when this attachment expires.
type CodexFrontend struct {
	ThreadCwd    string `json:"thread_cwd,omitempty"`
	Instance     string `json:"instance"`
	HomeID       string `json:"home_id"`
	SessionID    string `json:"session_id"`
	BridgePID    int    `json:"bridge_pid"`
	BridgeStart  string `json:"bridge_start"`
	StartedAt    int64  `json:"started_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Sequence     uint64 `json:"sequence"`
	BackendPID   int    `json:"backend_pid"`
	BackendStart string `json:"backend_start"`
	CodexBindingTarget
}

func frontendPath(instance string) string {
	if len(instance) != 32 || strings.Trim(instance, "0123456789abcdef") != "" {
		return ""
	}
	return filepath.Join(Dir(), "codex-frontends", instance+".json")
}

var frontendClients = codexterminal.Resolve
var frontendProcess = launchhost.ReadAgent
var frontendPaneView = tmux.Display

// frontendTarget always applies the strict foreground, TTY and birth checks,
// including when tmux reports an outer shell or gtmux as its current command.
func frontendTarget(pane string) (CodexBindingTarget, bool) {
	var panes []codexterminal.Pane
	for _, p := range codexProcessPanes() {
		if p.ID == pane {
			panes = append(panes, p)
		}
	}
	if len(panes) != 1 {
		return CodexBindingTarget{}, false
	}
	p := panes[0]
	c, ok := frontendClients(panes)[pane]
	if !ok {
		return CodexBindingTarget{}, false
	}
	return CodexBindingTarget{Pane: pane, Loc: p.Loc, Cwd: p.Cwd, PID: c.PID, PanePID: p.PID,
		ClientStart: c.Start, RootStart: c.RootStart, TTY: c.TTY}, true
}

// BeginCodexFrontend must be called only after the transport verified that its
// peer PID is this real Codex child. It performs no thread discovery or input.
func BeginCodexFrontend(pane, instance string, peerPID, backendPID int, sequence uint64) (CodexFrontend, error) {
	t, ok := frontendTarget(pane)
	b, exists := frontendProcess(os.Getpid())
	backend, backendExists := frontendProcess(backendPID)
	if frontendPath(instance) == "" || !ok || t.PID != peerPID || !exists || b.Start == "" || sequence == 0 || !backendExists || backend.Comm != "codex" || backend.Start == "" {
		return CodexFrontend{}, errors.New("frontend terminal identity unavailable")
	}
	now := time.Now().UnixMilli()
	return CodexFrontend{Instance: instance, HomeID: transcript.CodexHomeID(), BridgePID: b.PID,
		BridgeStart: b.Start, StartedAt: now, UpdatedAt: now, Sequence: sequence, BackendPID: backendPID, BackendStart: backend.Start, CodexBindingTarget: t}, nil
}

func validFrontend(f CodexFrontend) bool {
	return frontendPath(f.Instance) != "" && f.HomeID == transcript.CodexHomeID() && f.SessionID != "" &&
		len(f.SessionID) <= 256 && !strings.ContainsAny(f.SessionID, "\r\n\x00") &&
		f.Pane != "" && f.Loc != "" && f.PID > 1 && f.PanePID > 1 && f.BridgePID > 1 &&
		f.BridgeStart != "" && f.ClientStart != "" && f.RootStart != "" && f.TTY != "" &&
		f.StartedAt > 0 && f.UpdatedAt >= f.StartedAt && f.Sequence > 0 && f.BackendPID > 1 && f.BackendStart != ""
}

// BindCodexFrontend commits only a successful root-thread operation observed on
// that frontend's own connection. The caller excludes ephemeral/subagent work.
func BindCodexFrontend(f CodexFrontend, sid string, cwd ...string) (CodexFrontend, error) {
	if f.SessionID != sid {
		f.ThreadCwd = f.Cwd
	}
	f.SessionID, f.UpdatedAt = sid, time.Now().UnixMilli()
	if len(cwd) > 0 && filepath.IsAbs(cwd[0]) && !strings.ContainsAny(cwd[0], "\r\n\x00") {
		f.ThreadCwd = cwd[0]
	}
	if f.ThreadCwd == "" {
		f.ThreadCwd = f.Cwd
	}
	if !validFrontend(f) || !freshFrontend(f, true) {
		return f, errors.New("frontend changed before thread binding")
	}
	if err := state.MkdirAll(filepath.Dir(frontendPath(f.Instance)), 0700); err != nil {
		return f, err
	}
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		return f, err
	}
	defer lease.Close()
	previous, _ := Load(f.Loc)
	if previous.Frontend != "" && previous.Frontend != f.Instance {
		old, ok := readFrontend(previous.Frontend)
		if ok && (old.StartedAt > f.StartedAt || old.BridgePID == f.BridgePID && old.BridgeStart == f.BridgeStart && old.Sequence > f.Sequence) {
			return f, errors.New("a newer frontend owns this location")
		}
	}
	if !freshFrontend(f, true) {
		return f, errors.New("frontend changed while acquiring binding")
	}
	if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
		return f, err
	}
	r := Record{Agent: "codex", HomeID: f.HomeID, SessionID: sid, Cwd: f.ThreadCwd,
		UpdatedAt: f.UpdatedAt / 1000, Frontend: f.Instance}
	if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
		return f, err
	}
	if err := confirmFrontendStartup(f); err != nil {
		return f, &CodexBindingReceiptError{Err: err}
	}
	return f, nil
}

func confirmFrontendStartup(f CodexFrontend) error {
	observation, exists := native.Load(f.SessionID, f.HomeID)
	if !exists {
		return nil
	}
	return native.ConfirmBinding(native.Binding{Kind: "frontend", BackendPID: f.BackendPID, BackendStart: f.BackendStart,
		HomeID: f.HomeID, ID: f.Instance, SessionID: f.SessionID, Pane: f.Pane, Loc: f.Loc, Cwd: f.ThreadCwd,
		PID: f.PID, PanePID: f.PanePID, PreparedAt: f.StartedAt / 1000, BoundAt: f.UpdatedAt / 1000}, observation)
}

// RefreshCodexFrontend is a bounded liveness heartbeat from the bridge process.
// It does not promote a different thread or reclaim an incumbent's location.
func RefreshCodexFrontend(f CodexFrontend) (CodexFrontend, error) {
	f.UpdatedAt = time.Now().UnixMilli()
	if !validFrontend(f) || f.BridgePID != os.Getpid() {
		return f, errors.New("invalid frontend heartbeat")
	}
	r, ok := Load(f.Loc)
	if !ok || r.Agent != "codex" || r.SessionID != f.SessionID || r.HomeID != f.HomeID || r.Frontend != "" && r.Frontend != f.Instance {
		return f, errors.New("frontend binding replaced")
	}
	if r.Frontend == "" {
		// Older hook binaries omit the additive field when publishing the SAME
		// thread. Recover only from our own still-verified connection evidence.
		return BindCodexFrontend(f, f.SessionID, f.ThreadCwd)
	}
	if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
		return f, err
	}
	return f, confirmFrontendStartup(f)
}

func EndCodexFrontend(f CodexFrontend) {
	if f.BridgePID != os.Getpid() {
		return
	}
	if saved, ok := readFrontend(f.Instance); ok && saved.BridgePID == f.BridgePID && saved.BridgeStart == f.BridgeStart {
		// Keep the history explicitly non-live even if an older same-thread hook
		// rewrote its additive frontend field just before disconnect.
		if lease, err := state.TryLease("codex-frontend-binding"); err == nil {
			if r, exists := Load(f.Loc); exists && r.Agent == "codex" && r.SessionID == f.SessionID && r.HomeID == f.HomeID && r.Frontend == "" {
				r.Frontend = f.Instance
				_ = writeBindingJSON(fileFor(f.Loc), r)
			}
			lease.Close()
		}
		_ = os.Remove(frontendPath(f.Instance))
	}
}

func readFrontend(instance string) (CodexFrontend, bool) {
	var f CodexFrontend
	path := frontendPath(instance)
	if path == "" {
		return f, false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 || info.Mode().Perm()&0077 != 0 {
		return f, false
	}
	if owner, ok := info.Sys().(*syscall.Stat_t); !ok || owner.Uid != uint32(os.Geteuid()) {
		return f, false
	}
	file, err := state.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return f, false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return f, false
	}
	if json.NewDecoder(io.LimitReader(file, 8193)).Decode(&f) != nil {
		return f, false
	}
	return f, f.Instance == instance && validFrontend(f)
}

func freshFrontend(f CodexFrontend, strict bool) bool {
	age := time.Now().UnixMilli() - f.UpdatedAt
	if age < -1000 || age > 5000 {
		return false
	}
	p, ok := frontendProcess(f.BridgePID)
	if !ok || p.Start != f.BridgeStart {
		return false
	}
	if strict {
		backend, live := frontendProcess(f.BackendPID)
		if !live || backend.PID != f.BackendPID || backend.Start != f.BackendStart {
			return false
		}
		t, ok := frontendTarget(f.Pane)
		return ok && t == f.CodexBindingTarget
	}
	return frontendPaneView(f.Pane, "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}") ==
		f.Pane+"\t"+strconv.Itoa(f.PanePID)+"\t"+f.Loc
}

// LoadLive preserves legacy records while requiring live evidence for records
// produced by the frontend bridge. Restore continues to use Load/AllLocated.
func LoadLive(loc string) (Record, bool) {
	r, ok := Load(loc)
	if !ok || r.Frontend == "" {
		return r, ok
	}
	f, exists := readFrontend(r.Frontend)
	return r, exists && f.Loc == loc && f.HomeID == r.HomeID && f.SessionID == r.SessionID && freshFrontend(f, false)
}

// CodexFrontendForThread grants an exact current attachment only after a fresh
// terminal recheck. The runtime thread ID never becomes a pane ID by itself.
func CodexFrontendForThread(home, sid string) (CodexFrontend, bool) {
	var found CodexFrontend
	for _, r := range AllLocated() {
		if r.Agent != "codex" || r.HomeID != home || r.SessionID != sid || r.Frontend == "" {
			continue
		}
		f, ok := readFrontend(r.Frontend)
		if !ok || f.Loc != r.Loc || f.SessionID != sid || f.HomeID != home || !freshFrontend(f, true) {
			continue
		}
		if found.Instance != "" {
			return CodexFrontend{}, false
		}
		found = f
	}
	return found, found.Instance != ""
}

func CodexFrontendForPane(pane string) (CodexFrontend, bool) {
	loc := frontendPaneView(pane, "#{session_name}:#{window_index}.#{pane_index}")
	r, ok := LoadLive(loc)
	if !ok || r.Agent != "codex" || r.Frontend == "" {
		return CodexFrontend{}, false
	}
	f, ok := readFrontend(r.Frontend)
	return f, ok && f.Pane == pane && f.SessionID == r.SessionID && freshFrontend(f, true)
}

// VerifyDirectCodexCaller preserves the legacy independent-client route. A
// shared daemon's process can never satisfy the foreground frontend PID check.
func VerifyDirectCodexCaller(pane, home, sid string, pid int, birth string) bool {
	t, ok := frontendTarget(pane)
	if !ok || t.PID != pid || birth == "" {
		return false
	}
	p, ok := frontendProcess(pid)
	if !ok || p.PID != pid || p.Start != birth {
		return false
	}
	r, ok := LoadLive(t.Loc)
	return ok && r.Agent == "codex" && r.SessionID == sid && (r.HomeID == home || r.HomeID == "" && transcript.DefaultCodexHome())
}

// CodexCwdMatches keeps legacy cwd checks, but a bridged TUI can resume a
// different workspace while its supervising shell's physical cwd stays put.
func CodexCwdMatches(pane string, r Record, cwd string) bool {
	if r.Frontend == "" {
		return sameCanonicalCwd(r.Cwd, cwd)
	}
	f, ok := CodexFrontendForPane(pane)
	return ok && f.Instance == r.Frontend && f.SessionID == r.SessionID && sameCanonicalCwd(f.Cwd, cwd)
}
func sameCanonicalCwd(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, ea := filepath.EvalSymlinks(a)
	bb, eb := filepath.EvalSymlinks(b)
	if ea == nil && eb == nil {
		return aa == bb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
