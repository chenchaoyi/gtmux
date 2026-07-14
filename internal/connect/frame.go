package connect

import "encoding/json"

// The attach WebSocket wire protocol (ttyd-style): every binary frame's FIRST BYTE is
// an opcode; the payload follows from index 1. Raw PTY bytes travel unencoded in
// OUTPUT/INPUT payloads (no base64) so a real terminal renders them faithfully.
const (
	OpInput  byte = 'i' // client→server: raw key bytes for the pane
	OpResize byte = 'r' // client→server: JSON {"cols":C,"rows":R}
	OpPause  byte = 'p' // client→server: flow control — stop reading the PTY
	OpResume byte = 'R' // client→server: flow control — resume reading
	OpOutput byte = 'o' // server→client: raw PTY bytes
)

// Resize is the RESIZE frame payload.
type Resize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// Encode frames a payload with an opcode: [op][payload...].
func Encode(op byte, payload []byte) []byte {
	out := make([]byte, 1+len(payload))
	out[0] = op
	copy(out[1:], payload)
	return out
}

// Decode splits a frame into its opcode + payload. ok=false for an empty frame.
func Decode(frame []byte) (op byte, payload []byte, ok bool) {
	if len(frame) == 0 {
		return 0, nil, false
	}
	return frame[0], frame[1:], true
}

// EncodeResize frames a RESIZE with the given size.
func EncodeResize(cols, rows int) []byte {
	b, _ := json.Marshal(Resize{Cols: cols, Rows: rows})
	return Encode(OpResize, b)
}

// DecodeResize parses a RESIZE payload. ok=false on bad JSON or non-positive dims.
func DecodeResize(payload []byte) (cols, rows int, ok bool) {
	var r Resize
	if json.Unmarshal(payload, &r) != nil || r.Cols <= 0 || r.Rows <= 0 {
		return 0, 0, false
	}
	return r.Cols, r.Rows, true
}
