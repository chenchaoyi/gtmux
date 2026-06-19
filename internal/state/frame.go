package state

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Frame tracking gives a working-vs-idle signal for agents that don't animate a
// title spinner (e.g. Codex sets no pane title at all). We sample a pane's
// visible content each poll: a working agent's screen keeps changing (streaming,
// spinner, elapsed timer); an idle one sitting at its prompt is static. One file
// per pane under frame/, holding "<lastPoll> <lastChange> <hash>".
//
// This is a per-pane record, NOT part of the cross-process state contract the
// hook writes — it's purely an internal accelerator for `agents`.

// frameStaleSec: a baseline older than this means we weren't being polled (e.g.
// a one-shot `gtmux agents` long after the last), so the old frame is not a
// valid comparison — reset rather than false-positive "working".
const frameStaleSec = 6

// frameActiveSec: report "working" while a change was seen within this window.
// Bridges the gap between 1.5s polls (and a ~1Hz elapsed-timer tick) so a busy
// agent doesn't flicker idle, while a finished one settles to idle within ~4s.
const frameActiveSec = 4

// FrameDir holds the per-pane content-frame records.
func FrameDir() string { return filepath.Join(Dir(), "frame") }

func framePath(pane string) string { return filepath.Join(FrameDir(), pane) }

// frameHash is a cheap content fingerprint (cgo-free, collision risk irrelevant
// here — we only compare a pane against its own previous frame).
func frameHash(content string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	return strconv.FormatUint(h.Sum64(), 16)
}

// frameWorking is the pure decision: given the previous record and the current
// observation, return the new lastChange time and whether the pane is animating
// (working). Separated from disk I/O so it's unit-testable.
//
//   - No usable baseline (first sight, or the last poll was too long ago) →
//     record the frame but report idle; we need two consecutive recent samples.
//   - Otherwise the pane is working if its content changed now, or changed within
//     the last frameActiveSec.
func frameWorking(now, prevPoll, prevChange int64, prevHash, curHash string) (newChange int64, working bool) {
	if prevHash == "" || now-prevPoll > frameStaleSec {
		return 0, false
	}
	newChange = prevChange
	if curHash != prevHash {
		newChange = now
	}
	working = newChange != 0 && now-newChange < frameActiveSec
	return newChange, working
}

// PaneFrameWorking records pane's current visible content and reports whether the
// pane is actively changing (working) vs static (idle). `now` is unix seconds.
// First/stale observation returns false until a change is seen on a later poll.
func PaneFrameWorking(pane, content string, now int64) bool {
	var prevPoll, prevChange int64
	var prevHash string
	if b, err := os.ReadFile(framePath(pane)); err == nil {
		if f := strings.Fields(string(b)); len(f) == 3 {
			prevPoll, _ = strconv.ParseInt(f[0], 10, 64)
			prevChange, _ = strconv.ParseInt(f[1], 10, 64)
			prevHash = f[2]
		}
	}
	curHash := frameHash(content)
	newChange, working := frameWorking(now, prevPoll, prevChange, prevHash, curHash)

	if err := os.MkdirAll(FrameDir(), 0o755); err == nil {
		_ = os.WriteFile(framePath(pane), []byte(fmt.Sprintf("%d %d %s\n", now, newChange, curHash)), 0o644)
	}
	return working
}
