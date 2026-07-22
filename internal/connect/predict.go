package connect

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// Predictive local echo for `gtmux attach` (attach-predictive-echo stage 2). Typing over
// a slow link otherwise waits a full round-trip before each character echoes (~340 ms on
// the measured overseas tunnel). This draws the user's own printable keystrokes
// IMMEDIATELY, UNDERLINED to mark them unconfirmed, and erases them the moment
// authoritative output arrives — the server screen always wins.
//
// Reimplements mosh's DESIGN (its GPLv3 code is not copied), with one gtmux shortcut: we
// don't emulate a terminal to find the cursor — the server streams tmux's cursor as
// OpCursor (stage 1).
//
// Honesty rules, all of them load-bearing:
//   - printable ASCII + backspace ONLY; anything else ENDS the epoch (never predict
//     through a state change — the screen may jump arbitrarily);
//   - ADAPTIVE: predict only when the measured round-trip is slow enough to be worth
//     hiding (a LAN/fast tunnel shows nothing, so there is no flicker and no risk);
//   - NEVER on the alternate screen (a full-screen TUI) — `alt` from OpCursor;
//   - predictions are LOCAL ONLY: the real keystroke is forwarded to the pane untouched.

// predictThresholdMS is the round-trip above which hiding latency is worth it. Below it
// the echo already feels instant, so we draw nothing.
const predictThresholdMS = 50

// predictMaxPending caps how far ahead we will guess. Beyond a short run the odds of a
// clean single-line reconcile drop, so we stop predicting rather than risk the display.
const predictMaxPending = 64

type predictKind int

const (
	kindPrintable predictKind = iota
	kindBackspace
	kindStateChange // CR/LF/ESC/Ctrl-C/Tab/other control/multibyte — end the epoch
)

// classifyKey maps ONE raw input byte to what it means for prediction. Bytes >= 0x80 are
// multibyte UTF-8: their display width isn't 1 cell, so we refuse to guess and end the
// epoch instead of corrupting alignment.
func classifyKey(b byte) predictKind {
	switch {
	case b >= 0x20 && b <= 0x7e:
		return kindPrintable
	case b == 0x7f || b == 0x08:
		return kindBackspace
	default:
		return kindStateChange
	}
}

// Predictor holds one attach session's prediction state. Its methods return the bytes the
// caller should write to the terminal, so all terminal writes stay serialized by the
// caller's single stdout lock (the input and output goroutines both write).
type Predictor struct {
	mu       sync.Mutex
	enabled  bool   // --predict
	pending  []byte // predicted, not-yet-confirmed characters at the cursor
	alt      bool   // pane is on the alternate screen (full-screen TUI)
	haveCur  bool   // the server has sent at least one OpCursor
	epoch    bool   // false after a state-changing key, until the next OpCursor
	rttMS    float64
	lastSend time.Time
}

// NewPredictor builds a predictor. enabled=false (the default) makes every method a no-op
// returning no bytes, so attach behaves exactly as plain passthrough.
func NewPredictor(enabled bool) *Predictor {
	return &Predictor{enabled: enabled, epoch: true}
}

// predicting reports the full honesty gate. Caller must hold mu.
func (p *Predictor) predicting() bool {
	return p.enabled && p.epoch && p.haveCur && !p.alt &&
		p.rttMS >= predictThresholdMS && len(p.pending) < predictMaxPending
}

// OnInput is called AFTER the raw bytes have been forwarded to the pane (prediction never
// changes what is sent). It returns bytes to write to the local terminal: the underlined
// prediction, a rub-out for backspace, or an erase when a state-changing key ends the
// epoch. A chunk containing anything unpredictable ends the epoch for the whole chunk.
func (p *Predictor) OnInput(buf []byte) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.enabled {
		return nil
	}
	// Time every keystroke, even while not predicting — that's how rtt becomes known in
	// the first place (otherwise prediction could never switch on).
	if p.lastSend.IsZero() {
		p.lastSend = time.Now()
	}
	var out []byte
	for _, b := range buf {
		switch classifyKey(b) {
		case kindPrintable:
			if !p.predicting() {
				continue
			}
			p.pending = append(p.pending, b)
			out = append(out, underlined(b)...)
		case kindBackspace:
			if !p.predicting() {
				continue
			}
			if len(p.pending) == 0 {
				// We'd be erasing already-confirmed text — stop guessing, let the server do it.
				out = append(out, p.endEpochLocked()...)
				continue
			}
			p.pending = p.pending[:len(p.pending)-1]
			out = append(out, []byte("\x1b[1D \x1b[1D")...) // rub out one predicted cell
		default:
			out = append(out, p.endEpochLocked()...)
		}
	}
	return out
}

// OnOutput is called BEFORE authoritative server bytes are written to the terminal: it
// erases any outstanding predictions so the real output lands on a clean line. The server
// screen is authoritative — whatever we guessed is discarded here, right or wrong.
func (p *Predictor) OnOutput() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.erasePendingLocked()
}

// OnCursor folds in an authoritative cursor: it re-arms the epoch, records the alt-screen
// state, and closes the round-trip measurement that gates prediction.
func (p *Predictor) OnCursor(c Cursor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alt = c.Alt
	p.haveCur = true
	p.epoch = true
	if !p.lastSend.IsZero() {
		ms := float64(time.Since(p.lastSend).Milliseconds())
		if p.rttMS == 0 {
			p.rttMS = ms
		} else {
			p.rttMS = p.rttMS*0.7 + ms*0.3
		}
		p.lastSend = time.Time{}
	}
}

// endEpochLocked erases outstanding predictions and pauses prediction until the next
// authoritative cursor. Caller must hold mu.
func (p *Predictor) endEpochLocked() []byte {
	out := p.erasePendingLocked()
	p.epoch = false
	return out
}

// erasePendingLocked returns the sequence that removes exactly the predicted cells —
// back N, N spaces, back N. Deliberately NOT `ESC[K`, which would also wipe any real
// text to the right of the cursor. Caller must hold mu.
func (p *Predictor) erasePendingLocked() []byte {
	n := len(p.pending)
	if n == 0 {
		return nil
	}
	p.pending = p.pending[:0]
	back := "\x1b[" + strconv.Itoa(n) + "D"
	return []byte(back + strings.Repeat(" ", n) + back)
}

// underlined renders one predicted character as UNCONFIRMED. Only the underline attribute
// is toggled (not a full SGR reset), so we disturb the pane's own colors as little as
// possible — and mosh's underline is exactly the "don't be misled" signal.
func underlined(b byte) []byte {
	return []byte{0x1b, '[', '4', 'm', b, 0x1b, '[', '2', '4', 'm'}
}
