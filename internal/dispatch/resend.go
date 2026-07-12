package dispatch

import (
	"encoding/json"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// PayloadHash is a stable fingerprint of a delivery (pane + text). The re-send
// interlock keys on it so an IDENTICAL payload to the SAME pane is recognizable;
// a different pane or a single changed byte yields a different hash.
func PayloadHash(pane, text string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(pane))
	_, _ = h.Write([]byte{0}) // domain separator so pane|text can't collide across the boundary
	_, _ = h.Write([]byte(text))
	return strconv.FormatUint(h.Sum64(), 16)
}

// sendsDir is where per-pane recent-send records live.
func sendsDir() string { return filepath.Join(state.Dir(), "sends") }

// sendRecord is the persisted last-delivery for a pane.
type sendRecord struct {
	Hash string `json:"hash"`
	Ts   int64  `json:"ts"`
}

// sanitizePane makes a pane id safe as a filename ("%12" → "%12" is already fine,
// but a native session id could contain a slash). Only the final path element.
func sanitizePane(pane string) string {
	return filepath.Base(filepath.Clean("/" + pane))
}

// RecentSend returns the last-recorded payload hash and time for a pane
// (from the on-disk store), or ("", 0) when none. Real backend for Deliver's
// interlock; tests inject an in-memory equivalent.
func RecentSend(pane string) (hash string, ts int64) {
	b, err := os.ReadFile(filepath.Join(sendsDir(), sanitizePane(pane)+".json"))
	if err != nil {
		return "", 0
	}
	var r sendRecord
	if json.Unmarshal(b, &r) != nil {
		return "", 0
	}
	return r.Hash, r.Ts
}

// RecordSend persists the last payload hash + time for a pane (best-effort — a
// telemetry write must never fail a delivery).
func RecordSend(pane, hash string, ts int64) {
	if err := os.MkdirAll(sendsDir(), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(sendRecord{Hash: hash, Ts: ts})
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(sendsDir(), sanitizePane(pane)+".json"), b, 0o644)
}

// isDuplicate reports whether delivering `text` to `pane` now (unix `now`) would
// repeat the last payload within `window` seconds. window <= 0 disables the check.
// `recent` is the store lookup (injected in tests). This is the pure decision the
// interlock makes.
func isDuplicate(recent func(pane string) (string, int64), pane, text string, now, window int64) bool {
	if window <= 0 {
		return false
	}
	h := PayloadHash(pane, text)
	last, ts := recent(pane)
	return last == h && ts > 0 && now-ts < window
}
