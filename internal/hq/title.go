package hq

import (
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const hqWindowTitle = "gtmux HQ"
const hqNamedOption = "@gtmux_hq_named"
const hqPreviousNameOption = "@gtmux_hq_previous_name"

// nameHQWindow pins the visible tmux window name. With the supported
// set-titles-string (#S — #W), this also names the terminal tab. An existing
// manually named window belongs to its user; a new session belongs to gtmux.
func nameHQWindow(pane string, fresh bool) {
	if pane == "" || tmux.Display(pane, "#{pane_id}") != pane {
		return
	}
	if !fresh && tmux.Display(pane, "#{automatic-rename}") != "1" {
		return
	}
	name := hqWindowTitle + " " + pane
	previous := tmux.Display(pane, "#{window_name}")
	if !tmux.OK("set-window-option", "-t", pane, "automatic-rename", "off") {
		return
	}
	if !tmux.OK("rename-window", "-t", pane, name) {
		return
	}
	_, _ = tmux.Run("set-window-option", "-t", pane, hqNamedOption, name)
	_, _ = tmux.Run("set-window-option", "-t", pane, hqPreviousNameOption, previous)
}

// releaseHQWindow lets an old window resume its usual automatic name when HQ
// moves away. A user rename after our stamp is respected.
func releaseHQWindow(pane string) {
	name := tmux.Display(pane, "#{"+hqNamedOption+"}")
	if name == "" {
		return
	}
	if tmux.Display(pane, "#{window_name}") == name {
		previous := tmux.Display(pane, "#{"+hqPreviousNameOption+"}")
		if previous != "" {
			_, _ = tmux.Run("rename-window", "-t", pane, previous)
		}
		_, _ = tmux.Run("set-window-option", "-t", pane, "automatic-rename", "on")
	}
	_, _ = tmux.Run("set-window-option", "-u", "-t", pane, hqNamedOption)
	_, _ = tmux.Run("set-window-option", "-u", "-t", pane, hqPreviousNameOption)
}
