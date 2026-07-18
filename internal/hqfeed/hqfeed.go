// Package hqfeed is the perception feed (hq-attention-system): a gtmux-managed,
// LLM-free daemon that tails the session-event journal from a persisted CURSOR and
// spools every event to a rotated file the supervisor (HQ) subscribes to in the
// background — the SILENT channel that feeds HQ everything without typing visible
// lines into its pane.
//
// The split it enables: gtmux feeds HQ the full stream (here); HQ decides what to
// PRINT to the user. Robustness (the commander's #1 requirement) is layered:
//   - a monotonic seq + consumed cursor → zero-loss catch-up on restart,
//   - a 30s heartbeat a gtmux-side watchdog checks (stale 90s → mechanical restart),
//   - degradation (feed down / stale / cursor gap) surfaced as a CRITICAL control
//     record so a perception outage is known in seconds,
//   - startup reconciliation (replay from cursor + a control record telling HQ to
//     pull one full digest snapshot).
package hqfeed

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Timing constants — the commander's fixed N (design §6.4).
const (
	// HeartbeatInterval is how often the daemon proves it is alive.
	HeartbeatInterval = 30 * time.Second
	// StaleAfter is how old a heartbeat may get before the watchdog judges the feed
	// dead (3 missed beats —偏灵敏, tolerates a 2-beat blip).
	StaleAfter = 90 * time.Second
	// replayWindowSecs is the small Follow replay window that closes the race between
	// startup catch-up and the tail opening at end (seq>cursor dedup drops the overlap).
	replayWindowSecs = 5
)

// Control-record event names (written into the spool, not the journal). HQ's tail
// treats these specially: reconcile → pull a digest snapshot; feed-degraded →
// surface CRITICAL; self-check → run self-maintenance. They ride the same
// events.Record shape so HQ parses one line format.
const (
	ControlReconcile    = "gtmux:reconcile"
	ControlFeedDegraded = "gtmux:feed-degraded"
	// ControlWakeDegraded reports that the WAKE side is broken — knocks are queued
	// but never confirmed on HQ's screen. Its twin above covers the pull side going
	// dark; this covers the push side, and it must reach the pull stream precisely
	// because the channel that would otherwise announce it is the one that failed.
	ControlWakeDegraded = "gtmux:wake-degraded"
	ControlSelfCheck    = "gtmux:self-check"
	// ControlDistill asks HQ to run a periodic knowledge-distillation pass: distil the
	// fleet's event delta since the last distill into the knowledge base and prune
	// stale. A low-urgency maintenance signal like self-check (feed control record, not
	// a typed wake) — HQ does the curation; gtmux only raises it on a cadence.
	ControlDistill = "gtmux:distill"
)

// dir is the feed's private state home.
func dir() string { return filepath.Join(state.Dir(), "hq-feed") }

// PidPath / CursorPath / HeartbeatPath / SpoolPath name the feed's files.
func PidPath() string          { return filepath.Join(dir(), "pid") }
func CursorPath() string       { return filepath.Join(dir(), "cursor") }
func HeartbeatPath() string    { return filepath.Join(dir(), "heartbeat") }
func SpoolPath() string        { return filepath.Join(dir(), "spool.jsonl") }
func spoolRotatedPath() string { return filepath.Join(dir(), "spool.1.jsonl") }

// spoolCapBytes bounds the spool (rollable — the hard constraint). Active + one
// rotated generation ≈ 2×. 8 MB keeps HQ's context feed light.
const spoolCapBytes int64 = 8 << 20

// ReadCursor returns the last-consumed sequence (0 if none/unreadable).
func ReadCursor() int64 {
	n, _ := strconv.ParseInt(state.ReadMarker(CursorPath()), 10, 64)
	return n
}

// WriteCursor persists the last-consumed sequence (best-effort).
func WriteCursor(seq int64) { _ = state.WriteMarker(CursorPath(), strconv.FormatInt(seq, 10)) }

// Beat writes the heartbeat timestamp (best-effort). now is unix seconds.
func Beat(now int64) { _ = state.WriteMarker(HeartbeatPath(), strconv.FormatInt(now, 10)) }

// HeartbeatAt returns the last heartbeat's unix seconds (0 if none).
func HeartbeatAt() int64 {
	n, _ := strconv.ParseInt(state.ReadMarker(HeartbeatPath()), 10, 64)
	return n
}

// SpoolAppend writes one record to the spool, rotating FIRST when at/over the cap so
// the spool can never single-point-explode. Best-effort.
func SpoolAppend(r events.Record) {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return
	}
	if fi, err := os.Stat(SpoolPath()); err == nil && fi.Size() >= spoolCapBytes {
		_ = os.Rename(SpoolPath(), spoolRotatedPath())
	}
	line, err := json.Marshal(r)
	if err != nil {
		return
	}
	f, err := os.OpenFile(SpoolPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}

// EmitControl spools a synthetic control record (reconcile / feed-degraded /
// self-check) with the given human summary and severity, timestamped now.
func EmitControl(event, summary, severity string, now int64) {
	SpoolAppend(events.Record{Ts: now, Event: event, Summary: summary, Severity: severity})
}

// ReadSpool returns spool records from the last sinceSecs seconds (0 = all
// retained), oldest-first, spanning the rotated generation — the reader `--tail`
// and `--status` share.
func ReadSpool(sinceSecs, now int64) []events.Record {
	cutoff := int64(0)
	if sinceSecs > 0 {
		cutoff = now - sinceSecs
	}
	var out []events.Record
	for _, p := range []string{spoolRotatedPath(), SpoolPath()} {
		out = append(out, readSpoolFile(p, cutoff)...)
	}
	return out
}

func readSpoolFile(path string, cutoff int64) []events.Record {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []events.Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var r events.Record
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if cutoff > 0 && r.Ts < cutoff {
			continue
		}
		out = append(out, r)
	}
	return out
}

// --- pidfile singleton ------------------------------------------------------

// WritePid records the running daemon's pid.
func WritePid(pid int) error {
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(PidPath(), []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPid returns the recorded daemon pid (0 if none).
func ReadPid() int {
	n, _ := strconv.Atoi(strings.TrimSpace(state.ReadMarker(PidPath())))
	return n
}

// ProcessAlive reports whether a pid names a live process (signal 0 probe).
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// On unix, Signal 0 checks existence without delivering a signal.
	return syscall.Kill(pid, 0) == nil
}

// Running reports whether a live daemon currently holds the pidfile.
func Running() bool { return ProcessAlive(ReadPid()) }

// Stale reports whether the heartbeat is older than StaleAfter at `now` (unix secs).
// A missing heartbeat (0) is stale.
func Stale(now int64) bool {
	hb := HeartbeatAt()
	if hb == 0 {
		return true
	}
	return now-hb > int64(StaleAfter/time.Second)
}
