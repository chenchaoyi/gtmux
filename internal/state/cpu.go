package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CPU tracking is a hook-free working signal that COMPLEMENTS frame tracking
// (frame.go): a pane whose process subtree is burning CPU is working even if its
// screen isn't changing — e.g. a local tool (compile/test/grep) running quietly.
// We sample the subtree's cumulative CPU seconds each poll; a meaningful rise
// over the interval means it's actively computing. One file per pane under cpu/,
// holding "<lastPoll> <lastActive> <cumCPUSecs>".
//
// Caveat (by design): an LLM agent "thinking" is network-bound — the LOCAL
// process is idle while the model runs remotely — so CPU won't catch that; only
// the agent's own event/hook can. This signal is OR-combined with frame tracking,
// so it only ever turns idle→working (adds coverage, never adds flicker).
//
// Like frame.go, this is an internal accelerator for `agents`, NOT part of the
// cross-process state contract the hook writes.

// cpuStaleSec: a baseline older than this isn't a valid comparison (we weren't
// being polled) — reset rather than divide by a long interval.
const cpuStaleSec = 6

// cpuActiveSec: report working for this long after the last CPU-busy sample, so a
// busy subtree doesn't flicker idle between polls (mirrors frameActiveSec).
const cpuActiveSec = 4

// cpuBusyRate: CPU seconds per wall second that count as "actively computing".
// 0.30 = ~a third of one core sustained over the interval — well clear of an
// idle process's noise, comfortably tripped by a real local tool.
const cpuBusyRate = 0.30

// CPUDir holds the per-pane CPU-sample records.
func CPUDir() string { return filepath.Join(Dir(), "cpu") }

func cpuPath(pane string) string { return filepath.Join(CPUDir(), pane) }

// cpuWorking is the pure decision: given the previous record and the current
// cumulative CPU, return the new lastActive time and whether the subtree is
// actively computing. prevCPU < 0 means "no baseline".
//   - No usable baseline, stale, or non-advancing clock → record, report idle.
//   - Otherwise busy if CPU rose at >= cpuBusyRate now, or did within cpuActiveSec.
func cpuWorking(now, prevPoll, prevActive int64, prevCPU, curCPU float64) (newActive int64, working bool) {
	if prevCPU < 0 || now-prevPoll > cpuStaleSec || now <= prevPoll {
		return 0, false
	}
	newActive = prevActive
	if rate := (curCPU - prevCPU) / float64(now-prevPoll); rate >= cpuBusyRate {
		newActive = now
	}
	working = newActive != 0 && now-newActive < cpuActiveSec
	return newActive, working
}

// PaneCPUWorking records the pane subtree's current cumulative CPU seconds and
// reports whether it's actively computing (working) vs quiescent. `now` is unix
// seconds. First/stale observation returns false until a rise is seen later.
func PaneCPUWorking(pane string, curCPU float64, now int64) bool {
	prevCPU := -1.0
	var prevPoll, prevActive int64
	if b, err := os.ReadFile(cpuPath(pane)); err == nil {
		if f := strings.Fields(string(b)); len(f) == 3 {
			prevPoll, _ = strconv.ParseInt(f[0], 10, 64)
			prevActive, _ = strconv.ParseInt(f[1], 10, 64)
			if v, err := strconv.ParseFloat(f[2], 64); err == nil {
				prevCPU = v
			}
		}
	}
	newActive, working := cpuWorking(now, prevPoll, prevActive, prevCPU, curCPU)

	if err := os.MkdirAll(CPUDir(), 0o755); err == nil {
		_ = os.WriteFile(cpuPath(pane), []byte(fmt.Sprintf("%d %d %.2f\n", now, newActive, curCPU)), 0o644)
	}
	return working
}
