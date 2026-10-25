package resume

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var codexProcessClients = codexterminal.Resolve
var codexProcessIdentity = launchhost.ReadAgent
var codexProcessInteractive = codexterminal.InteractiveMode
var codexProcessPanes = func() []codexterminal.Pane {
	var panes []codexterminal.Pane
	for _, line := range tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_path}\t#{session_name}:#{window_index}.#{pane_index}") {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			continue
		}
		pid, _ := strconv.Atoi(f[2])
		panes = append(panes, codexterminal.Pane{ID: f[0], Command: f[1], PID: pid, TTY: f[3], Cwd: f[4], Loc: f[5]})
	}
	return panes
}

func processCLIIdentity(scope *transcript.CodexPathScope, r native.Record, now int64) bool {
	if r.Agent != "codex" || r.HomeID != transcript.CodexHomeID() || r.SessionID == "" || r.PID <= 1 ||
		r.Comm != "codex" || r.ProcessStart == "" || r.Event == "SessionEnd" || r.Event == "" || r.UpdatedAt <= 0 || r.UpdatedAt > now+1 {
		return false
	}
	id, ok := scope.CodexIdentityFor(r.SessionID)
	return ok && id.HomeID == r.HomeID && id.CreationClient == "terminal" && id.Source == "cli" &&
		hqpane.SameDir(id.Cwd, r.Cwd)
}

func processTarget(r native.Record, panes []codexterminal.Pane) (CodexBindingTarget, bool) {
	clients := codexProcessClients(panes)
	var target CodexBindingTarget
	for _, p := range panes {
		client, ok := clients[p.ID]
		if !ok || client.PID != r.PID || client.Start == "" || client.RootStart == "" || client.TTY == "" || !hqpane.SameDir(r.Cwd, p.Cwd) {
			continue
		}
		if target.Pane != "" && target.Pane != p.ID {
			return CodexBindingTarget{}, false
		}
		target = CodexBindingTarget{Pane: p.ID, Loc: p.Loc, Cwd: p.Cwd, PID: client.PID, PanePID: p.PID,
			ClientStart: client.Start, RootStart: client.RootStart, TTY: client.TTY, Launcher: agents.LauncherFromCommand(p.Command, "codex")}
	}
	if target.Pane == "" || target.Loc == "" {
		return CodexBindingTarget{}, false
	}
	// The hook stores UTC process birth. The terminal inspector's own birth
	// strings may use the host timezone, so compare through the same UTC reader.
	process, ok := codexProcessIdentity(r.PID)
	return target, ok && process.PID == r.PID && process.Comm == r.Comm && process.Start == r.ProcessStart && codexProcessInteractive(r.PID)
}

// commitProcessBinding preserves the observation and compares both the incumbent
// and live frontend again under the location's existing binding lease. It never
// fabricates a submitted-task witness or retires conversation history.
func commitProcessBinding(r native.Record, target CodexBindingTarget, incumbent Record, stored bool, now int64) error {
	if target.Pane == "" || target.Loc == "" || target.PID != r.PID || now < r.UpdatedAt ||
		incumbent.SessionID != r.SessionID && incumbent.UpdatedAt >= r.UpdatedAt {
		return errors.New("process observation predates the current binding")
	}
	// A new identity must be a real conversation. Deferring an empty startup is
	// safer than letting a helper or command stub claim a foreground client.
	if incumbent.SessionID != r.SessionID {
		turns, err := transcript.Load("codex", r.SessionID, 1)
		if err != nil || len(turns) == 0 {
			return errors.New("process observation is not a conversation")
		}
	}
	if err := state.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	lockPath := fileFor(target.Loc) + ".binding-lock"
	if info, err := os.Stat(lockPath); err == nil && time.Since(info.ModTime()) > time.Minute {
		_ = os.Remove(lockPath)
	}
	lock, err := state.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	lock.Close()
	defer os.Remove(lockPath)
	current, _ := Load(target.Loc)
	if current != incumbent {
		return errors.New("process binding acquired a newer owner")
	}
	if stored {
		if current, ok := native.Load(r.SessionID, r.HomeID); !ok || current != r {
			return errors.New("process observation changed")
		}
	}
	fresh, ok := processTarget(r, codexProcessPanes())
	if !ok || target != fresh {
		return errors.New("terminal process or pane changed")
	}
	if current, _ := Load(target.Loc); current != incumbent {
		return errors.New("process binding acquired a newer owner")
	}
	if stored {
		if current, ok := native.Load(r.SessionID, r.HomeID); !ok || current != r {
			return errors.New("process observation changed during verification")
		}
	}
	record := Record{Agent: "codex", HomeID: r.HomeID, SessionID: r.SessionID, Cwd: target.Cwd, UpdatedAt: now, Launcher: target.Launcher}
	if incumbent.SessionID == r.SessionID && incumbent.HomeID == r.HomeID {
		record.TerminalClient = incumbent.TerminalClient
	}
	if err := writeBindingJSON(fileFor(target.Loc), record); err != nil {
		return err
	}
	diag.For("resume").Info("codex.binding.process.confirmed", "verified foreground terminal client bound to its exact session", "pane", target.Pane, "agent_session", r.SessionID, "pid", r.PID)
	return nil
}

// BindCodexHookProcess admits a fresh main-session start/submit callback from its
// actual foreground terminal client. Shared or headless app-server stdin fails
// the terminal inspector. Callers must retain submitted-nonce validation first.
func BindCodexHookProcess(sid, cwd, event string, process launchhost.AgentProcess, now int64) string {
	if (event != "SessionStart" && event != "UserPromptSubmit") || process.ObservedAt <= 0 || now-process.ObservedAt > 60 || process.ObservedAt > now {
		return ""
	}
	r := native.Record{Agent: "codex", HomeID: transcript.CodexHomeID(), SessionID: sid, Cwd: cwd, PID: process.PID,
		Comm: process.Comm, ProcessStart: process.Start, Event: event, UpdatedAt: process.ObservedAt}
	if !processCLIIdentity(transcript.NewCodexPathScope(), r, now) {
		return ""
	}
	target, ok := processTarget(r, codexProcessPanes())
	if !ok {
		return ""
	}
	incumbent, _ := Load(target.Loc)
	if err := commitProcessBinding(r, target, incumbent, false, now); err != nil {
		return ""
	}
	return target.Pane
}

// ReconcileCodexProcesses repairs stored, unambiguous hook observations without
// a new user turn. Only the current radar's admitted pane locations may change.
func ReconcileCodexProcesses(scope *transcript.CodexPathScope, records []native.Record, locations map[string]string, now int64) {
	if scope == nil || len(locations) == 0 {
		return
	}
	var candidates []native.Record
	claims := map[int]map[string]bool{}
	for _, r := range records {
		if !processCLIIdentity(scope, r, now) {
			continue
		}
		if claims[r.PID] == nil {
			claims[r.PID] = map[string]bool{}
		}
		claims[r.PID][r.SessionID] = true
		candidates = append(candidates, r)
	}
	var unbound []native.Record
	for _, r := range candidates {
		if len(claims[r.PID]) != 1 {
			continue
		}
		bound := false
		for _, loc := range locations {
			current, _ := Load(loc)
			bound = bound || current.Agent == "codex" && current.HomeID == r.HomeID && current.SessionID == r.SessionID
		}
		if !bound {
			unbound = append(unbound, r)
		}
	}
	if len(unbound) == 0 {
		return
	}
	panes := codexProcessPanes()
	for _, r := range unbound {
		target, ok := processTarget(r, panes)
		if !ok || locations[target.Pane] != target.Loc {
			continue
		}
		incumbent, _ := Load(target.Loc)
		if err := commitProcessBinding(r, target, incumbent, true, now); err != nil {
			diag.For("resume").Info("codex.binding.process.deferred", "terminal observation no longer proves an unchanged binding", "pane", target.Pane, "agent_session", r.SessionID, "error", err)
		}
	}
}
