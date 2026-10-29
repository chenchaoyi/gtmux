package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func terminalInputTarget(id string) (terminalkeys.Target, error) {
	if len(id) < 2 || id[0] != '%' || strings.Trim(id[1:], "0123456789") != "" {
		return terminalkeys.Target{}, fmt.Errorf("invalid pane")
	}
	if tmux.Bin == "" {
		return terminalkeys.Target{}, fmt.Errorf("pane unavailable")
	}
	identity := tmux.Display(id, "#{pane_id}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_command}\t#{session_id}\t#{session_name}:#{window_index}.#{pane_index}")
	parts := strings.Split(identity, "\t")
	if len(parts) != 6 || parts[0] != id {
		return terminalkeys.Target{}, fmt.Errorf("pane unavailable")
	}
	rec, _ := resume.LoadLive(parts[5])
	// Persisted conversation identity alone is not a live route. The pane's current
	// process/tty/session observation must match as well when sending.
	sum := sha256.Sum256([]byte(identity + "\x00" + rec.Agent + "\x00" + rec.HomeID + "\x00" + rec.SessionID))
	t := terminalkeys.Target{Binding: hex.EncodeToString(sum[:])}
	if rec.Agent == "codex" && strings.Contains(strings.ToLower(parts[3]), "codex") {
		if screen, err := tmux.CapturePaneChecked(id); err == nil {
			t.Actions = terminalkeys.CodexQuestions("codex", screen, t.Binding)
		}
	}
	return t, nil
}

func sendTerminalKey(id, binding string, event terminalkeys.Event, action *terminalkeys.ActionRequest) error {
	key, err := terminalkeys.Encode(event)
	if err != nil {
		return err
	}
	t, err := terminalInputTarget(id)
	if err != nil || t.Binding != binding {
		return fmt.Errorf("pane input route changed")
	}
	if action != nil {
		match := false
		for _, a := range t.Actions {
			if a.ID == action.ID && a.Revision == action.Revision {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("question context changed")
		}
	}
	_ = tmux.ExitCopyMode(id)
	if bytes, ok := terminalkeys.CharacterBytes(event); ok {
		args := []string{"send-keys", "-t", id, "-H"}
		for _, b := range bytes {
			args = append(args, fmt.Sprintf("%02x", b))
		}
		_, err = tmux.Run(args...)
		return err
	}

	return tmux.SendKey(id, key)
}
