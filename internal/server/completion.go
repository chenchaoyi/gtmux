package server

import "time"

func sameNotificationSession(a, b AgentStatus) bool {
	return a.PaneID == b.PaneID && a.Agent == b.Agent && a.Loc == b.Loc &&
		a.SessionID == b.SessionID && a.HomeID == b.HomeID && a.Client == b.Client && a.Instance == b.Instance
}

// completionReady remembers observed work across unconfirmed idle polls. The
// completion generation prevents a repaint from re-alerting an already ended turn.
// Only tick's serial goroutine owns these maps.
func (h *hub) completionReady(a, prev AgentStatus, existed bool) bool {
	if h.pendingDone == nil {
		h.pendingDone = make(map[string]AgentStatus)
		h.lastDone = make(map[string]string)
	}
	key := a.key()
	if !existed || !sameNotificationSession(a, prev) {
		delete(h.pendingDone, key)
		delete(h.lastDone, key)
	}
	eligible := a.Role != "supervisor" && !a.StatusOnly && (!a.explicitNotifyConsent() || a.Notify)
	if !eligible || a.EndReason != "" || a.Status == "running" {
		delete(h.pendingDone, key)
	} else if a.Status == "working" {
		// Re-observing the same working state must not move the turn's start.
		if _, armed := h.pendingDone[key]; !armed || prev.Status != "working" {
			h.pendingDone[key] = a
		}
	}
	if a.Status != "idle" || !a.Confirmed {
		return false
	}
	work, armed := h.pendingDone[key]
	fresh := a.CompletionID == "" || a.CompletionID != h.lastDone[key]
	// A completion predating the observed turn belongs to history, even if the
	// server started while a display-only repaint looked like work.
	current := a.CompletedAt == 0 || work.Since == 0 || a.CompletedAt >= work.Since*int64(time.Second)
	h.lastDone[key] = a.CompletionID
	if eligible && armed && fresh && current && a.EndReason == "" && sameNotificationSession(a, work) {
		delete(h.pendingDone, key)
		return true
	}
	return false
}
