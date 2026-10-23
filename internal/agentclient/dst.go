package agentclient

import (
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// CandidatePane is only an input refusal guard. Unverified client discovery
// cannot label a harness or authorize input, including alternate dst backends.
func CandidatePane(pane string) bool {
	inspect := productionInspector()
	panePID, _, err := inspect.Pane(pane)
	if err != nil {
		return false
	}
	var servers []struct {
		PID int `json:"pid"`
	}
	if readOwnedJSON(filepath.Join(state.Home(), ".dsh-tui", "inject", "servers.json"), &servers, 256<<10) != nil || len(servers) > 256 {
		return false
	}
	for _, s := range servers {
		parent, command, err := inspect.Process(s.PID)
		if err != nil || !dstProcess(command) {
			continue
		}
		if s.PID == panePID {
			return true
		}
		seen := map[int]bool{}
		for depth := 0; parent > 1 && depth < 32 && !seen[parent]; depth++ {
			if parent == panePID {
				return true
			}
			seen[parent] = true
			parent, _, err = inspect.Process(parent)
			if err != nil {
				break
			}
		}
	}
	return false
}

// MatchesSavedClient is client liveness evidence only. A saved DSH resume record
// must independently supply the verified harness/home/session identity.
func MatchesSavedClient(client, command string) bool {
	if client != "dst" || !savedDSTProcess(command) {
		return false
	}
	fields := strings.Fields(command)
	for i, field := range fields {
		if field == "--backend" && (i+1 >= len(fields) || fields[i+1] != "dsh") {
			return false
		}
		if backend, ok := strings.CutPrefix(field, "--backend="); ok && backend != "dsh" {
			return false
		}
	}
	return true
}

// The save sees the outer CLI (often `node /.../bin/dst`), not its
// nested host. This is evidence only when paired with a verified resume record;
// live attachment verification deliberately keeps the stricter dstProcess gate.
func savedDSTProcess(command string) bool {
	if dstProcess(command) {
		return true
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	if filepath.Base(fields[0]) == "dst" {
		return true
	}
	return len(fields) > 1 && (filepath.Base(fields[0]) == "node" || filepath.Base(fields[0]) == "nodejs") && filepath.Base(fields[1]) == "dst"
}

func dstProcess(command string) bool {
	return strings.Contains(command, "/@deepseek-harness-tui/dsh-tui/lib/types/dsh-adapter/host-entry.js") ||
		strings.Contains(command, "/@deepseek-ai/dsh/lib/bin.js") && strings.Contains(command, "--profile dsh-tui")
}

// dst publishes every mounted session separately from its current injection
// endpoint. Both must agree; a background mount never owns the foreground pane.
func dstCurrent(a Attachment) bool {
	root := filepath.Join(state.Home(), ".dsh-tui")
	var servers []struct {
		PID       int    `json:"pid"`
		SessionID string `json:"sessionId"`
		StartedAt int64  `json:"startedAt"`
	}
	if readOwnedJSON(filepath.Join(root, "inject", "servers.json"), &servers, 256<<10) != nil || len(servers) > 256 {
		return false
	}
	matching := 0
	for _, s := range servers {
		if s.PID == a.PID {
			if s.SessionID != a.SessionID || s.StartedAt != a.BindingEpoch {
				return false
			}
			matching++
		}
	}
	if matching != 1 {
		return false
	}
	var mounts struct {
		Version int `json:"version"`
		Owners  []struct {
			PID        int      `json:"pid"`
			SessionIDs []string `json:"sessionIds"`
		} `json:"owners"`
	}
	if readOwnedJSON(filepath.Join(root, "session-mounts.json"), &mounts, 256<<10) != nil || mounts.Version != 1 || len(mounts.Owners) > 256 {
		return false
	}
	found := false
	matchingOwners := 0
	for _, owner := range mounts.Owners {
		if owner.PID != a.PID {
			continue
		}
		matchingOwners++
		if len(owner.SessionIDs) > 256 {
			return false
		}
		for _, id := range owner.SessionIDs {
			if id == a.SessionID {
				found = true
			} else if strings.HasSuffix(id, ":"+a.SessionID) {
				return false
			}
		}
	}
	return found && matchingOwners == 1
}
