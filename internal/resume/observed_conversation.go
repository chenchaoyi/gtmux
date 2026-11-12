package resume

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// ConversationObservation is a display/read-only view. It is deliberately not a
// Record or Attachment and cannot be passed to any input or restore resolver.
type ConversationObservation struct {
	SessionID string `json:"session_id"`
	HomeID    string `json:"home_id"`
	State     string `json:"state"` // recovered | last-known; neither is input authority
}

// ObservedCodexConversations folds independently evidenced historical views into
// admitted pane presentations. Missing proof stays unknown; names, directories,
// inherited pane hints and shared backend processes never identify a pane.
func ObservedCodexConversations(scope *transcript.CodexPathScope, locations map[string]string, now int64) map[string]ConversationObservation {
	result := map[string]ConversationObservation{}
	if len(locations) == 0 {
		return result
	}
	if scope == nil {
		scope = transcript.NewCodexPathScope()
	}
	home := transcript.CodexHomeID()
	unbound := map[string]string{}
	for pane, loc := range locations {
		if r, live := LoadLive(loc); live && r.SessionID != "" {
			continue // The ordinary current identity always wins.
		}
		if r, recovered, err := ConversationRecord(loc); err == nil && recovered && r.Agent == "codex" && r.HomeID == home {
			result[pane] = ConversationObservation{r.SessionID, r.HomeID, "recovered"}
			continue
		}
		unbound[pane] = loc
	}
	if len(unbound) == 0 {
		return result
	}
	bindings, err := native.PrivateBindingObservations(home)
	if err != nil || len(bindings) == 0 {
		return result
	}
	var panes []codexterminal.Pane
	for _, p := range codexProcessPanes() {
		if unbound[p.ID] == p.Loc {
			panes = append(panes, p)
		}
	}
	clients := codexProcessClients(panes)
	claims := map[string][]native.Binding{}
	for _, b := range bindings {
		c, ok := clients[b.Pane]
		if !ok || unbound[b.Pane] != b.Loc || c.PID != b.PID {
			continue
		}
		for _, p := range panes {
			if p.ID == b.Pane && p.PID == b.PanePID && p.Loc == b.Loc {
				claims[b.Pane] = append(claims[b.Pane], b)
				break
			}
		}
	}
	for pane, receipts := range claims {
		// Resolve competing history before filtering its metadata. An unknown
		// newer thread must not make the old thread look uniquely current.
		if len(receipts) != 1 {
			continue
		}
		b, c := receipts[0], clients[pane]
		if b.Kind != "frontend" || frontendPath(b.ID) == "" || b.BackendPID <= 1 || b.BackendStart == "" || b.BoundAt > now ||
			c.TTY == "" || !observedBirth(c.Start, b.PreparedAt, now) || !observedBirth(c.RootStart, b.PreparedAt, now) || !codexProcessInteractive(b.PID) {
			continue
		}
		if r, ok := Load(b.Loc); ok && (r.Agent != "codex" || r.HomeID != home || r.SessionID != b.SessionID || r.Frontend != "" && r.Frontend != b.ID) {
			continue
		}
		identity, ok := scope.CodexIdentityFor(b.SessionID)
		if !ok || identity.HomeID != home || identity.CreationClient != "terminal" || (identity.Source != "cli" && identity.Source != "vscode") {
			continue
		}
		result[pane] = ConversationObservation{b.SessionID, home, "last-known"}
	}
	return result
}

func observedBirth(snapshot string, prepared, now int64) bool {
	// Resolve already compared two kernel snapshots, terminal foreground group,
	// real stdin and pane location. These local birth times additionally reject
	// PID reuse since the independent receipt; same-second births are ambiguous.
	birth, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", snapshot, time.Local)
	return err == nil && birth.Unix() > 0 && birth.Unix() < prepared && prepared <= now
}
