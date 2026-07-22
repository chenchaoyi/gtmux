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
	// OpCursor carries the bridged tmux pane's cursor + alt-screen flag
	// (attach-predictive-echo): the AUTHORITATIVE cursor the client would otherwise
	// need a terminal emulator to derive. gtmux can send it because the server bridges
	// a tmux pane, and tmux knows the cursor. Additive — an old client ignores it.
	OpCursor byte = 'c' // server→client: JSON {"x":C,"y":R,"alt":bool}
)

// Resize is the RESIZE frame payload.
type Resize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// Cursor is the CURSOR frame payload: the pane's cursor cell plus whether the pane is
// on the ALTERNATE screen (a full-screen TUI), where predictive echo must stand down.
type Cursor struct {
	X   int  `json:"x"`
	Y   int  `json:"y"`
	Alt bool `json:"alt"`
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

// EncodeCursor frames a CURSOR with the pane's cursor cell + alt-screen flag.
func EncodeCursor(x, y int, alt bool) []byte {
	b, _ := json.Marshal(Cursor{X: x, Y: y, Alt: alt})
	return Encode(OpCursor, b)
}

// DecodeCursor parses a CURSOR payload. ok=false on bad JSON or a negative cell (a
// negative coordinate can't address a cell, so it's treated as "unknown").
func DecodeCursor(payload []byte) (c Cursor, ok bool) {
	if json.Unmarshal(payload, &c) != nil || c.X < 0 || c.Y < 0 {
		return Cursor{}, false
	}
	return c, true
}
