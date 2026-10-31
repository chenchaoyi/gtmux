package state

import (
	"encoding/json"
	"os"
	"time"
)

// Completion is lifecycle evidence stored in the finished marker's payload.
// Its timestamp is independent of mtime, which the radar may repair for display.
// Legacy empty markers and inferred idle-time caches are deliberately not proof.
type Completion struct {
	Version   int    `json:"version"`
	Agent     string `json:"agent"`
	SessionID string `json:"session_id,omitempty"`
	TurnID    string `json:"turn_id,omitempty"`
	At        int64  `json:"at"` // Unix nanoseconds; also identifies this completion
}

func RecordCompletion(pane, agent, sessionID, turnID string, at time.Time) error {
	c := Completion{Version: 1, Agent: agent, SessionID: sessionID, TurnID: turnID, At: at.UnixNano()}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	path := FinishedPath(pane)
	if err := WritePrivateFile(path, b); err != nil {
		return err
	}
	return os.Chtimes(path, at, at)
}

func ReadCompletion(pane string) (Completion, bool) {
	var c Completion
	b := ReadMarker(FinishedPath(pane))
	if json.Unmarshal([]byte(b), &c) != nil || c.Version != 1 || c.Agent == "" || c.At <= 0 || c.At > time.Now().Add(time.Second).UnixNano() {
		return Completion{}, false
	}
	return c, true
}
