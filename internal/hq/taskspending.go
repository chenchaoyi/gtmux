// The PENDING-DECISION STANDING VIEW (hq-signal-ergonomics §4) — `gtmux tasks --pending`.
//
// It exists because a list was being simulated in a scrolling transcript: every brief of
// one 400+-turn HQ shift re-printed the same unchanged 「你手上未变: A · B · C…」 line. A
// set that is re-stated verbatim on every read is a VIEW, and a view belongs somewhere it
// can be read on demand instead of pushed repeatedly at a reader who has already seen it.
//
// Two properties follow from that, and both are load-bearing:
//
//   - It reads the LEDGER ONLY — no radar join. A standing view is polled, and a full
//     `radar.GatherAgents()` runs a process scan (the wedged-`ps` incident froze the menu
//     bar for exactly this reason). What is on the commander's plate is also not a
//     question about pane state: an item stays pending while its pane works, idles, or
//     dies.
//   - It carries NO CLOCK. A relative "waiting 3h" would make two reads differ with
//     nothing changed, which is precisely the churn the view was built to end; the entry
//     stamp is absolute, so the bytes move only when the SET does.
package hq

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// pendingRow is one plate entry: the ledger row plus the wait clock it is ordered by
// (resolved once, in dispatch, so the sort key and the printed stamp cannot drift).
type pendingRow struct {
	row   taskJSON
	since int64
}

// pendingTasks returns the entries awaiting the commander in the view's STABLE order:
// decision grade first (the tier projection — same scale the wake lines carry), then the
// oldest wait first, then pane, then id. The last two are not cosmetic: they make the
// order total, so the view cannot wobble between two reads of an unchanged set.
//
// Archived entries are excluded by definition — archiving IS closure, and a closed item
// is off the plate.
func pendingTasks(tasks []dispatch.Task, now int64) []pendingRow {
	var out []pendingRow
	for _, t := range tasks {
		if t.Archived || !t.AwaitingCommander() {
			continue
		}
		out = append(out, pendingRow{row: rowFor(t, "pending", now), since: t.PendingSince()})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ga, gb := hqwake.GradeOfTier(a.row.Tier), hqwake.GradeOfTier(b.row.Tier)
		switch {
		case ga != gb:
			return ga > gb // decision before attention before ledger
		case a.since != b.since:
			return a.since < b.since // oldest wait first
		case a.row.Pane != b.row.Pane:
			return a.row.Pane < b.row.Pane
		default:
			return a.row.ID < b.row.ID
		}
	})
	return out
}

// pendingJSON unwraps the view's rows for `--pending --json`.
func pendingJSON(rows []pendingRow) []taskJSON {
	out := make([]taskJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.row)
	}
	return out
}

// pendingStamp renders the wait clock as an ABSOLUTE local time — data, not a countdown.
func pendingStamp(since int64) string {
	if since <= 0 {
		return i18n.PadRight("—", 11)
	}
	return time.Unix(since, 0).Format("01-02 15:04")
}

// renderPending writes the standing view. Colour is passed in (never sniffed here) so a
// test can pin the exact bytes both ways; the glyph carries the grade on its own, so a
// pipe loses nothing.
func renderPending(w io.Writer, rows []pendingRow, color bool) {
	if len(rows) == 0 {
		fmt.Fprintln(w, i18n.Tr("Nothing is awaiting your decision.", "没有待你决定的事项。"))
		return
	}
	for _, r := range rows {
		g := hqwake.GradeOfTier(r.row.Tier)
		loc := r.row.Pane
		if r.row.Session != "" {
			loc = r.row.Session + " " + r.row.Pane
		}
		fmt.Fprintf(w, "%s %s %s %s  %s\n",
			g.Paint(g.Glyph(), color),
			i18n.PadRight(r.row.ID, 14),
			i18n.PadRight(loc, 22),
			pendingStamp(r.since),
			r.row.Goal)
	}
}

// markPending puts an entry on the plate / takes it off, and reports the outcome on the
// same surface the view lives on. `disposition` is empty for a mark; for a resolve it
// records HOW the item left (decided / withdrawn / escalated …).
func markPending(id string, await bool, disposition string, now int64) int {
	if id == "" {
		i18n.Sae("gtmux tasks: missing <task_id>", "gtmux tasks: 缺少 <task_id>")
		return 2
	}
	ok := false
	if await {
		ok = dispatch.MarkAwaitingCommander(id, now)
	} else {
		ok = dispatch.ClearAwaitingCommander(id, disposition, now)
	}
	if !ok {
		i18n.Sae("gtmux tasks: no ledger entry '"+id+"'", "gtmux tasks: 账本里没有 '"+id+"'")
		return 1
	}
	if await {
		i18n.Say("awaiting commander: "+id, "已挂待决："+id)
		return 0
	}
	tail := disposition
	if tail == "" {
		tail = i18n.Tr("cleared", "已清")
	}
	i18n.Say("off the plate: "+id+" ("+tail+")", "已下架："+id+"（"+tail+"）")
	return 0
}
