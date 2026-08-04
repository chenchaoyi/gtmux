package hq

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Choosing which agent runs HQ.
//
// `gtmux hq` used to always type `claude` into the fresh supervisor pane, so a user
// whose machine is signed into Codex (but not Claude) got an HQ session stuck on "Not
// logged in · Please run /login". The launch now RESOLVES an agent — the --agent flag,
// then the GTMUX_HQ_AGENT env override, then a REMEMBERED choice, then (at an interactive
// terminal, when nothing has been chosen yet) an interactive PICKER of the agents that
// are actually installed — and remembers the pick so the user is asked at most once.

// hqAgentChoicePath is where the remembered HQ agent lives (the picker's result, or an
// explicit --agent). Read on the next `gtmux hq` so the user isn't asked again.
func hqAgentChoicePath() string { return filepath.Join(state.HQHome(), ".gtmux-hq-agent") }

func readHQAgentChoice() string {
	b, err := os.ReadFile(hqAgentChoicePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeHQAgentChoice(cmd string) error {
	c := strings.TrimSpace(cmd)
	if c == "" {
		return nil
	}
	return os.WriteFile(hqAgentChoicePath(), []byte(c+"\n"), 0o644)
}

// resolveHQLaunchAgent decides which agent command to launch (or relaunch) HQ with.
// Precedence: the --agent flag (remembered, an explicit choice) › the GTMUX_HQ_AGENT env
// override (transient — not remembered) › the remembered choice › an interactive picker
// of the installed agents (remembered) › claude. The picker only appears at an
// interactive terminal and only when nothing has been chosen yet, so `gtmux hq` from a
// script never blocks; and because this runs only on a spawn/relaunch (never when
// focusing a live HQ), focusing an existing supervisor never prompts.
func resolveHQLaunchAgent(flagAgent string) string {
	if a := strings.TrimSpace(flagAgent); a != "" {
		_ = writeHQAgentChoice(a)
		return a
	}
	if a := strings.TrimSpace(os.Getenv("GTMUX_HQ_AGENT")); a != "" {
		return a
	}
	if a := readHQAgentChoice(); a != "" {
		return a
	}
	if a := pickHQAgent(os.Stdin, os.Stderr); a != "" {
		_ = writeHQAgentChoice(a)
		return a
	}
	return hqAgentCommand() // env already handled above → the "claude" default
}

// hqAgentCandidate is one selectable agent for HQ: a hook-equipped agent whose launch
// binary is on PATH.
type hqAgentCandidate struct {
	key   string
	label string
	cmd   string // the binary to type to start a fresh session
}

// hqLaunchBinary is the bare binary that starts a FRESH session for an agent (its resume
// binary run with no resume args, a detect command, else the key).
func hqLaunchBinary(m agents.Manifest) string {
	switch {
	case len(m.Resume) > 0:
		return m.Resume[0]
	case len(m.Detect) > 0:
		return m.Detect[0]
	default:
		return m.Key
	}
}

// detectHQAgents lists the hook-equipped agents whose launch binary resolves via lookPath
// — the agents that can run as a supervisor AND feed HQ its wake/event stream. lookPath is
// injected so detection is testable without a real PATH.
func detectHQAgents(lookPath func(string) (string, error)) []hqAgentCandidate {
	var out []hqAgentCandidate
	seen := map[string]bool{}
	for _, m := range agents.All() {
		if !m.Hooked {
			continue // HQ needs the event stream to wake — hook-equipped agents only
		}
		bin := hqLaunchBinary(m)
		if bin == "" || seen[bin] {
			continue
		}
		if _, err := lookPath(bin); err != nil {
			continue
		}
		seen[bin] = true
		out = append(out, hqAgentCandidate{key: m.Key, label: m.Label, cmd: bin})
	}
	return out
}

// hqAgentDefaultIndex is the pre-selected candidate: claude when present (the most common
// supervisor), else the first.
func hqAgentDefaultIndex(cands []hqAgentCandidate) int {
	for i, c := range cands {
		if c.key == "claude" {
			return i
		}
	}
	return 0
}

// pickHQAgent offers an interactive choice of the installed agents and returns the chosen
// launch command. It returns "" (so the caller falls back to the default) when stdin is
// not a terminal or nothing is detected; it auto-picks when exactly one agent is installed
// (no choice to make).
func pickHQAgent(in *os.File, out io.Writer) string {
	if in == nil || !term.IsTerminal(int(in.Fd())) {
		return ""
	}
	cands := detectHQAgents(exec.LookPath)
	if len(cands) == 0 {
		return ""
	}
	if len(cands) == 1 {
		return cands[0].cmd
	}
	def := hqAgentDefaultIndex(cands)
	fmt.Fprintln(out, i18n.Tr("Which agent should run HQ (中控)?", "HQ（中控）用哪个 agent 来跑？"))
	for i, c := range cands {
		mark := "  "
		if i == def {
			mark = "→ "
		}
		fmt.Fprintf(out, "  %s%d) %s\n", mark, i+1, c.label)
	}
	fmt.Fprint(out, i18n.Tr(
		fmt.Sprintf("Choose 1-%d [default %d]: ", len(cands), def+1),
		fmt.Sprintf("选择 1-%d [默认 %d]：", len(cands), def+1)))
	line, _ := bufio.NewReader(in).ReadString('\n')
	return chooseHQAgent(cands, def, line)
}

// chooseHQAgent maps a picker reply to a launch command: a valid 1-based index picks that
// agent; empty or invalid input takes the default. Pure, for testing.
func chooseHQAgent(cands []hqAgentCandidate, def int, reply string) string {
	if len(cands) == 0 {
		return ""
	}
	if def < 0 || def >= len(cands) {
		def = 0
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return cands[def].cmd
	}
	if n, err := strconv.Atoi(reply); err == nil && n >= 1 && n <= len(cands) {
		return cands[n-1].cmd
	}
	return cands[def].cmd
}
