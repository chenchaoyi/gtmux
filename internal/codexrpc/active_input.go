package codexrpc

import "encoding/json"

// An empty turn ID means a positively identified legacy/idle queue path. A
// failure is not proof that a turn is idle and must not trigger another send.
func activeInputTurn(thread string, id int, send func(int, string, any) error, read func(int) (wireResponse, error)) (string, int, string) {
	if send(id, "thread/read", map[string]any{"threadId": thread, "includeTurns": false}) != nil {
		return "", id, "input-metadata-unavailable"
	}
	r, e := read(id)
	id++
	if e != nil {
		return "", id, "input-metadata-unavailable"
	}
	if r.Error != nil {
		if r.Error.Code == -32601 || r.Error.Code == -32602 {
			return "", id, ""
		}
		return "", id, "input-metadata-rejected"
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
		return "", id, "input-metadata-mismatch"
	}
	if len(meta.Thread.Status.Flags) > 0 {
		return "", id, "input-awaiting-confirmation"
	}
	switch meta.Thread.Status.Type {
	case "idle", "notLoaded":
		return "", id, ""
	case "active":
	default:
		return "", id, "input-state-unavailable"
	}
	if meta.Thread.Direct == nil || !*meta.Thread.Direct {
		return "", id, ""
	}
	if send(id, "thread/turns/list", map[string]any{"threadId": thread, "limit": 1, "sortDirection": "desc", "itemsView": "notLoaded"}) != nil {
		return "", id, "input-turn-unavailable"
	}
	r, e = read(id)
	id++
	if e != nil {
		return "", id, "input-turn-unavailable"
	}
	if r.Error != nil {
		if r.Error.Code == -32601 || r.Error.Code == -32602 {
			return "", id, ""
		}
		return "", id, "input-turn-rejected"
	}
	var page struct {
		Data []struct {
			ID, Status string
			Items      []json.RawMessage `json:"items"`
			View       string            `json:"itemsView"`
		} `json:"data"`
	}
	if json.Unmarshal(r.Result, &page) != nil || len(page.Data) != 1 {
		return "", id, "input-turn-unavailable"
	}
	t := page.Data[0]
	if t.Status != "inProgress" || t.ID == "" || len(t.ID) > 256 {
		return "", id, "input-turn-changed"
	}
	if t.View != "notLoaded" || len(t.Items) > 0 {
		return "", id, "input-turn-view-unavailable"
	}
	return t.ID, id, ""
}
