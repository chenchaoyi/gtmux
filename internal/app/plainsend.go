package app

import (
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Plain-terminal send routing (fix/plain-pane-send). The paste→box-confirm→submit
// pipeline (dispatch.PasteAndSubmit / dispatch.Deliver) verifies a delivery against an
// AGENT's input box. A PLAIN terminal pane — a bash/zsh pane opened via the pane
// browser — has no input box, and running the box-confirm against one false-fails
// whenever the pane's scrollback holds stale box-drawing (a pane that USED to run an
// agent keeps the agent's old input-box borders in the 200-line capture margin):
// SplitInputRegion locks onto the stale box ABOVE the live prompt, the paste's echo at
// the real prompt BELOW it is invisible to the detector, and the C-u clear-retry wipes
// the user's line — the phone showed "Not sent — the input box didn't confirm the full
// message" for a send into a bare shell. Plain panes therefore take a DIRECT
// type+Enter path; agent panes keep the box-confirm pipeline unchanged.

// resolvePaneAgent resolves the agent command driving a pane — process SUBTREE first
// (Claude renames its process to its version, so pane_current_command misses "claude";
// see radar.AgentDriverKey), foreground command as the fallback — and reports whether
// the pane is a PLAIN terminal that must take the direct type path.
func resolvePaneAgent(id string) (agentCmd string, plain bool) {
	if k := radar.AgentDriverKey(id); k != "" {
		return k, false // a known agent drives the pane
	}
	fg := tmux.Display(id, "#{pane_current_command}")
	return fg, plainTermPane("", fg)
}

// plainTermPane is the pure routing predicate: a pane is a PLAIN terminal only when
// NO known agent was found in its process subtree (driverKey empty) AND the
// foreground command is a bare shell. Anything else — an agent, a version-named
// foreground like Claude's "2.1.220" (the subtree scan missed it), vim, ssh — fails
// SAFE to the agent pipeline, which is the pre-fix behavior for those panes.
func plainTermPane(driverKey, fgCmd string) bool {
	return driverKey == "" && dispatchbridge.ShellCommands[fgCmd]
}

// typePlain delivers text+Enter to a plain terminal pane directly: single-line text
// as literal keystrokes (`send-keys -l`), multi-line via the bracketed paste buffer
// (so a shell with bracketed-paste doesn't run each line early), then Enter. No box
// confirm — there is no box; the shell's own echo is the confirmation the caller's
// screen snapshot carries. The caller is responsible for exiting copy-mode first.
func typePlain(id, text string) error {
	if keystrokeText(text) {
		if err := tmux.SendText(id, text, false); err != nil {
			return err
		}
	} else if text != "" {
		if err := tmux.Paste(id, text); err != nil {
			return err
		}
	}
	return tmux.SendKey(id, "Enter")
}
