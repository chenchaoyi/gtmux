package hq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// Reading the supervisor's own knowledge for REMOTE surfaces (hq-command-page).
//
// A remote HQ page has, without these, nothing to show that the radar doesn't already
// show — so it ends up re-listing the fleet. What the supervisor uniquely holds is its
// situation BOARD (the synthesis it maintains by hand, so its picture survives a context
// reset) and the severity-tagged event LEDGER (history, where the radar only has the
// present instant). Both are read-only here: the board is the supervisor's working
// memory, and gtmux is not an editor for it.

// boardMaxBytes caps what we hand a client. The board is prose a person maintains, so it
// stays small in practice; the cap exists so a runaway file can't be pushed down a phone
// tunnel. Truncation keeps the HEAD — a board leads with its freshness line and current
// focus, which is exactly what a reader needs.
const boardMaxBytes = 128 << 10

// boardEventsWindow bounds how far back the ledger read scans. The feed shows recent
// activity, and reading the entire retained log on every poll would be wasteful for
// records no client will render.
const boardEventsWindow = 24 * 3600

// actsWindow is how far back an acts-only read looks. The supervision's own acts are
// sparse — measured 39 in a week against 1532 wake deliveries — so a week of them is a
// short list, and the phone's "this week" tally was counting a 24-hour window while
// saying week (2026-09-14: 「这些信息只展示两天的历史」).
const actsWindow = 7 * 24 * 3600

// BoardPath is the supervisor's situation board.
func BoardPath() string { return filepath.Join(hqNotesDir(), "board.md") }

// Board returns the situation board's text and when it was last written. ok=false means
// no board exists — an ordinary state for a supervisor that has never written one (and
// for a machine with no HQ at all), NOT an error.
func Board() (text string, modUnix int64, ok bool) {
	p := BoardPath()
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return "", 0, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", 0, false
	}
	if len(b) > boardMaxBytes {
		b = b[:boardMaxBytes]
	}
	return string(b), fi.ModTime().Unix(), true
}

// EventsJSON returns the event ledger as a marshaled array at a severity FLOOR
// ("" = every tier), NEWEST FIRST and capped at limit records — the shape a feed reads,
// as opposed to the CLI's oldest-first log order. It always returns a valid JSON array,
// never null, so a client can render "no activity" without special-casing.
//
// actsOnly narrows it to what the SUPERVISION did (events.IsSupervisorAct). The filter
// belongs on this side because the acts are sparse in a feed the wake plumbing dominates:
// measured on a real machine, the 200-record cap spanned 3.9 hours and carried 5 acts
// where the day held 37. A client filtering after the fact cannot show a week.
func EventsJSON(minSeverity string, limit int, actsOnly bool) ([]byte, error) {
	if limit <= 0 {
		return []byte("[]"), nil
	}
	minRank := events.SeverityRank(minSeverity)
	window := int64(boardEventsWindow)
	if actsOnly {
		window = actsWindow
	}
	all := events.Read(window, time.Now().Unix())
	// Walk backwards: the newest records are the ones a feed shows, and stopping at the
	// cap means a long window costs no more than a short one to marshal.
	out := make([]events.Record, 0, limit)
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		if minSeverity != "" && events.SeverityRank(all[i].Severity) < minRank {
			continue
		}
		if actsOnly && !events.IsSupervisorAct(all[i]) {
			continue
		}
		out = append(out, all[i])
	}
	return json.Marshal(out)
}
