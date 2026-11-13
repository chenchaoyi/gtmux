package codexrpc

import "encoding/json"

// startIdle is permitted only for explicit user ingress, never background queueing.
func inputTurnPlan(thread string, id int, send func(int, string, any) error, read func(int) (wireResponse, error), userInput bool) (string, bool, int, string) {
	if send(id, "thread/read", map[string]any{"threadId": thread, "includeTurns": false}) != nil {
		return "", false, id, "input-metadata-unavailable"
	}
	r, e := read(id)
	id++
	if e != nil {
		return "", false, id, "input-metadata-unavailable"
	}
	if r.Error != nil {
		if r.Error.Code == -32601 || r.Error.Code == -32602 {
			return "", false, id, ""
		}
		return "", false, id, "input-metadata-rejected"
	}
	var meta struct {
		Thread struct {
			ID     string `json:"id"`
			Direct *bool  `json:"canAcceptDirectInput"`
			Status struct {
				Type  string   `json:"type"`
				Flags []string `json:"activeFlags"`
			} `json:"status"`
		} `json:"thread"`
	}
	if json.Unmarshal(r.Result, &meta) != nil || meta.Thread.ID != thread {
		return "", false, id, "input-metadata-mismatch"
	}
	if len(meta.Thread.Status.Flags) > 0 {
		for _, flag := range meta.Thread.Status.Flags {
			if flag != "waitingOnApproval" && flag != "waitingOnUserInput" {
				// Unknown flags still refuse input, without inventing a human decision.
				return "", false, id, "input-state-unavailable"
			}
		}
		return "", false, id, "input-awaiting-confirmation"
	}
	switch meta.Thread.Status.Type {
	case "notLoaded":
		if userInput {
			return "", false, id, "input-thread-not-loaded"
		}
		return "", false, id, ""
	case "idle":
		return "", userInput && meta.Thread.Direct != nil && *meta.Thread.Direct, id, ""
	case "active":
	default:
		return "", false, id, "input-state-unavailable"
	}
	if meta.Thread.Direct == nil || !*meta.Thread.Direct {
		return "", false, id, ""
	}
	if send(id, "thread/turns/list", map[string]any{"threadId": thread, "limit": 1, "sortDirection": "desc", "itemsView": "notLoaded"}) != nil {
		return "", false, id, "input-turn-unavailable"
	}
	r, e = read(id)
	id++
	if e != nil {
		return "", false, id, "input-turn-unavailable"
	}
	if r.Error != nil {
		if r.Error.Code == -32601 || r.Error.Code == -32602 {
			return "", false, id, ""
		}
		return "", false, id, "input-turn-rejected"
	}
	var page struct {
		Data []struct {
			ID, Status string
			Items      []json.RawMessage `json:"items"`
			View       string            `json:"itemsView"`
		} `json:"data"`
	}
	if json.Unmarshal(r.Result, &page) != nil || len(page.Data) != 1 {
		return "", false, id, "input-turn-unavailable"
	}
	t := page.Data[0]
	if t.Status != "inProgress" || t.ID == "" || len(t.ID) > 256 {
		return "", false, id, "input-turn-changed"
	}
	if t.View != "notLoaded" || len(t.Items) > 0 {
		return "", false, id, "input-turn-view-unavailable"
	}
	return t.ID, false, id, ""
}
