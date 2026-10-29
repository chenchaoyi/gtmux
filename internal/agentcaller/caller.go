// Package agentcaller resolves the executing conversation without borrowing a shared backend environment.
package agentcaller

import (
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type Identity struct {
	Agent, HomeID, ThreadID, Pane, Session, PanePID string
	Subagent                                        bool
	ProcessPID                                      int
	ProcessStart                                    string
}

var callerHQPane = hqpane.Find
var callerPaneDriver = radar.AgentDriverKey
var callerPanes = func() []string {
	return tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{session_name}\t#{pane_pid}")
}
var callerProcess = func() launchhost.AgentProcess {
	var names []string
	for name := range agents.CommandKeys() {
		names = append(names, name)
	}
	return launchhost.ObserveAgent(names)
}

// Codex supplies the executing thread ID for each tool invocation. TMUX_PANE
// can instead belong to the shared daemon's first client. It is never a fallback
// when an explicit thread ID is invalid or has no verified current attachment.
func runtimeCodexThread() (Identity, bool, bool) {
	sid, legacy := os.Getenv("CODEX_THREAD_ID"), os.Getenv("CODEX_SESSION_ID")
	if sid == "" {
		sid = legacy
	}
	if sid == "" {
		return Identity{}, false, true
	}
	process := callerProcess()
	if process.Comm != "" && agents.CommandKeys()[process.Comm] != "codex" {
		return Identity{}, false, true
	}
	id, ok := transcript.CodexIdentityFor(sid)
	if !ok || id.HomeID != transcript.CodexHomeID() || !id.Subagent && ((id.Source != "cli" && id.Source != "vscode") || (id.CreationClient != "terminal" && id.Source != "cli")) {
		return Identity{}, true, false
	}
	return Identity{Agent: "codex", HomeID: id.HomeID, ThreadID: sid, Subagent: id.Subagent, ProcessPID: process.PID, ProcessStart: process.Start}, true, true
}

func Current() (Identity, bool) {
	c, runtime, valid := runtimeCodexThread()
	if runtime {
		if !valid {
			return c, false
		}
		if c.Subagent {
			c.Session = "Codex " + c.ThreadID
			return c, true
		}
		for _, line := range callerPanes() {
			f := strings.Split(line, "\t")
			if len(f) != 4 {
				continue
			}
			r, exists := resume.LoadLive(f[1])
			if !exists || r.Agent != "codex" || r.SessionID != c.ThreadID || r.HomeID != "" && r.HomeID != c.HomeID || r.HomeID == "" && !transcript.DefaultCodexHome() {
				continue
			}
			if callerPaneDriver(f[0]) != "codex" {
				continue
			}
			if active := state.ReadMarker(state.ActivePath(f[0])); active != "" && active != c.ThreadID {
				continue
			}
			if r.Frontend != "" {
				verified, ok := resume.CodexFrontendForThread(c.HomeID, c.ThreadID)
				if !ok || verified.Pane != f[0] {
					continue
				}
			}
			if c.Pane != "" {
				c.Pane, c.Session, c.PanePID = "", "Codex "+c.ThreadID, ""
				return c, true
			}
			c.Pane, c.Session, c.PanePID = f[0], f[2], f[3]
		}
		if c.Session == "" {
			c.Session = "Codex " + c.ThreadID
		}
		return c, true
	}
	pane := os.Getenv("TMUX_PANE")
	for _, line := range callerPanes() {
		f := strings.Split(line, "\t")
		if len(f) == 4 && f[0] == pane {
			return Identity{Pane: pane, Session: f[2], PanePID: f[3]}, true
		}
	}
	return Identity{}, false
}

func IsHQ() bool {
	c, ok := Current()
	return ok && c.Pane != "" && c.Pane == callerHQPane()
}

func DiagnosticActor() string {
	if os.Getenv("CODEX_THREAD_ID") == "" && os.Getenv("CODEX_SESSION_ID") == "" {
		return ""
	}
	switch os.Getenv("GTMUX_ACTOR") {
	case "menubar", "system", "update":
		return ""
	}
	c, ok := Current()
	if !ok || c.Pane == "" {
		return "system"
	}
	if c.Pane == callerHQPane() {
		return "hq"
	}
	return "agent:" + c.Pane
}

// Pane returns only a currently resolved caller location.
func Pane() string {
	if !HasCodexRuntime() {
		return os.Getenv("TMUX_PANE")
	}
	c, ok := Current()
	if !ok {
		return ""
	}
	return c.Pane
}

// HasCodexRuntime distinguishes explicit tool identity from ordinary shell invocations.
func HasCodexRuntime() bool { _, present, _ := runtimeCodexThread(); return present }
