// Post-restore reconciliation: after resurrect has put the sessions back, gtmux checks
// that what came back is what was saved, and says so when it isn't.
//
// It exists because BOTH sides of the restore were silent about layout. gtmux only ever
// counted session NAMES (waitForRestoredSessions), never looked at the windows; and
// tmux-resurrect runs its `restore_window_properties` with `>/dev/null 2>&1`, so when
// `select-layout` refuses a window (classically "have 3 panes but need 2" — the restored
// window ended up with one pane more than the save recorded), the failure is discarded
// where nobody can see it and that window is simply left in the default stacked
// arrangement it was built with. Neither half speaks, so a broken layout is discovered
// only by the person looking at it, days later, with no evidence left.
//
// Two windows on this user's machine did exactly that (2026-08-15 and 2026-08-18), each
// time with one extra pane. This does not FIX the extra pane — the cause is still open —
// but it ends the silence: the drift is named in restore.log and on the terminal at the
// moment it happens, while the evidence is fresh.
package app

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// windowShape is one window's ARRANGEMENT: how many panes it holds and how they are
// divided. It is what a person means by "my layout", stripped of everything volatile.
type windowShape struct {
	Session string
	Index   string
	Panes   int
	Layout  string // normalized (see normalizeLayout) — comparable across a restore
}

// key identifies the window across the save/live boundary.
func (w windowShape) key() string { return w.Session + ":" + w.Index }

// layoutChecksumRe matches the 4-hex checksum tmux prefixes onto a layout string, which
// is also how a layout FIELD is recognized positionally-independently.
var layoutChecksumRe = regexp.MustCompile(`^[0-9a-f]{4},\d+x\d+,\d+,\d+`)

// layoutLeafRe matches a leaf cell's trailing pane number — `WxH,X,Y,<pane>`. A CONTAINER
// cell is `WxH,X,Y` followed by `{` or `[`, so it never matches and its geometry survives.
var layoutLeafRe = regexp.MustCompile(`(\d+x\d+,\d+,\d+),\d+`)

// normalizeLayout reduces a tmux layout string to the part that means "this is how the
// window is divided", so a saved layout and a restored one can be compared at all.
//
// Two things in the raw string change on every restore and must go:
//
//   - the leading 4-hex CHECKSUM, which covers the whole string including the pane
//     numbers below, so it differs whenever they do;
//   - the PANE NUMBER each leaf cell ends with. Those are tmux pane ids, and a restarted
//     server reissues them from %1 — the same arrangement comes back numbered
//     differently, and a byte comparison would call every single window "changed".
//
// What survives is the geometry and the nesting — `189x48,0,0{94x48,0,0,94x48,95,0}` —
// which is exactly the side-by-side-vs-stacked, how-wide-is-each-half question a person
// is asking when they say the layout came back wrong.
func normalizeLayout(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ','); i == 4 && layoutChecksumRe.MatchString(s) {
		s = s[5:]
	}
	return layoutLeafRe.ReplaceAllString(s, "$1")
}

// savedShapes reads the window arrangement a resurrect save recorded: one entry per
// `window` line, with the pane count taken from that window's `pane` lines (the save's
// window line carries no count, and the count is the half that actually breaks).
//
// The layout field is located by SHAPE, not by index. Resurrect's own re-read of its dump
// collapses runs of tabs, so an empty field shifts everything after it left by one — the
// documented hazard the pane parser in restoresave.go handles the same way. A layout is
// the only field that looks like `<4 hex>,<W>x<H>,<x>,<y>`, so matching it is immune to
// however the line shifted.
func savedShapes(path string) []windowShape {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []windowShape
	at := map[string]int{} // key → index into out
	panes := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) < 3 || fields[1] == "" {
			continue
		}
		switch fields[0] {
		case "window":
			w := windowShape{Session: fields[1], Index: fields[2]}
			for _, fld := range fields[3:] {
				if layoutChecksumRe.MatchString(fld) {
					w.Layout = normalizeLayout(fld)
					break
				}
			}
			if _, dup := at[w.key()]; !dup {
				at[w.key()] = len(out)
				out = append(out, w)
			}
		case "pane":
			panes[fields[1]+":"+fields[2]]++
		}
	}
	for i := range out {
		out[i].Panes = panes[out[i].key()]
	}
	return out
}

// liveShapes reads the same arrangement from the running server.
func liveShapes() []windowShape {
	var out []windowShape
	for _, line := range tmux.Lines("list-windows", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{window_panes}\t#{window_layout}") {
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			continue
		}
		n, _ := strconv.Atoi(f[2])
		out = append(out, windowShape{Session: f[0], Index: f[1], Panes: n, Layout: normalizeLayout(f[3])})
	}
	return out
}

// layoutDrift compares what was saved against what came back and returns one localized
// line per window that differs — pane count first, because a pane-count mismatch is what
// makes `select-layout` refuse the window outright and is therefore the CAUSE of most
// layout drift, not another symptom of it.
//
// A saved window with no live counterpart is reported as missing. The reverse (a live
// window the save never had) is deliberately NOT reported: sessions the user created
// after the save are normal and expected, and flagging them would bury the real signal.
// Pure — no tmux, no files — so the comparison is unit-testable.
func layoutDrift(saved, live []windowShape) []string {
	byKey := map[string]windowShape{}
	for _, w := range live {
		byKey[w.key()] = w
	}
	var out []string
	for _, s := range saved {
		l, ok := byKey[s.key()]
		if !ok {
			out = append(out, i18n.Tr(
				fmt.Sprintf("%s: window did not come back", s.key()),
				fmt.Sprintf("%s: 这扇窗没回来", s.key())))
			continue
		}
		switch {
		case s.Panes != l.Panes:
			out = append(out, i18n.Tr(
				fmt.Sprintf("%s: saved with %d pane(s), came back with %d — tmux refuses to apply a layout to the wrong number of panes, so this window kept the default arrangement", s.key(), s.Panes, l.Panes),
				fmt.Sprintf("%s: 存档里 %d 个窗格,回来是 %d 个 —— 窗格数对不上时 tmux 不会套用布局,这扇窗停在了默认排布", s.key(), s.Panes, l.Panes)))
		case s.Layout != "" && s.Layout != l.Layout:
			out = append(out, i18n.Tr(
				fmt.Sprintf("%s: arrangement differs — saved %s, now %s", s.key(), s.Layout, l.Layout),
				fmt.Sprintf("%s: 排布和存档不一样 —— 存档 %s,现在 %s", s.key(), s.Layout, l.Layout)))
		}
	}
	return out
}

// driftReportMax is how many drift lines are printed to the TERMINAL. All of them go to
// restore.log; the terminal gets a bounded head plus a count, because a restore that went
// badly wrong should not bury the one line telling you where the save is.
const driftReportMax = 5

// reportLayoutDrift compares the save against the live server and reports every window
// that came back differently — to restore.log always, and to the user when there is
// something to say. Silent when everything matches, which is the normal case.
func reportLayoutDrift(save string) {
	if save == "" {
		return
	}
	saved := savedShapes(save)
	if len(saved) == 0 {
		restoreLogf("layout-check: save has no window lines — nothing to reconcile")
		return
	}
	live := liveShapes()
	if len(live) == 0 {
		restoreLogf("layout-check: no live windows to compare against — skipped")
		return
	}
	drift := layoutDrift(saved, live)
	if len(drift) == 0 {
		restoreLogf("layout-check: %d saved window(s) all came back with the same panes and arrangement", len(saved))
		return
	}
	restoreLogf("layout-check: %d of %d saved window(s) came back DIFFERENT:\n  %s",
		len(drift), len(saved), strings.Join(drift, "\n  "))
	head := i18n.Tr(
		fmt.Sprintf("⚠ %d of %d restored window(s) don't match the save:", len(drift), len(saved)),
		fmt.Sprintf("⚠ %d/%d 扇窗恢复得和存档不一样:", len(drift), len(saved)))
	i18n.Sae(head, head)
	for i, d := range drift {
		if i == driftReportMax {
			more := i18n.Tr(
				fmt.Sprintf("  … and %d more — full list in %s", len(drift)-driftReportMax, restoreLogPath()),
				fmt.Sprintf("  …… 还有 %d 条 —— 完整列表见 %s", len(drift)-driftReportMax, restoreLogPath()))
			i18n.Sae(more, more)
			break
		}
		i18n.Sae("  "+d, "  "+d)
	}
}

// saveAgeNote is the one line that says WHICH moment is being restored: the save's
// timestamp and how old it is.
//
// The existing staleness warning only fires past a day, and a day is far too coarse for
// the failure that actually costs work. The reboot that prompted this restored a save
// taken 37 minutes before shutdown — well inside every threshold, and yet everything the
// user did in those 37 minutes (including the layout change they later reported as
// "restore broke my layout") was simply not in the file. Printing the age turns that from
// an invisible fact into an obvious one. Pure (path + now in, string out).
func saveAgeNote(lastPath string, now time.Time) string {
	age, ok := saveAge(lastPath, now)
	if !ok {
		return ""
	}
	when := now.Add(-age).Format("15:04")
	return i18n.Tr(
		fmt.Sprintf("Restoring the layout saved at %s (%s ago) — anything you changed after that is not in it.", when, shortAge(age)),
		fmt.Sprintf("恢复的是 %s 存下的布局(%s前)—— 那之后的改动不在里面。", when, shortAge(age)))
}

// shortAge renders a restore-scale age as "37m" / "5h" / "3d".
func shortAge(d time.Duration) string {
	switch secs := int64(d.Seconds()); {
	case secs < 60:
		return "<1m"
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 24*3600:
		return fmt.Sprintf("%dh", secs/3600)
	default:
		return fmt.Sprintf("%dd", secs/(24*3600))
	}
}

// afterRestore is the reconciliation step every successful restore runs, before the
// conversations are brought back.
//
// Two jobs, both about the same thing — the state gtmux is carrying across the reboot
// no longer matches the machine:
//
//   - Drop pane-keyed records whose panes are gone. This is the first moment they are
//     not just stale but WRONG: the server restarted, so it is reissuing pane ids, and
//     yesterday's records now name today's panes. Doing it here, before any hook fires,
//     means nothing gets to read a cross-wired record even once.
//   - Report layout drift, while the evidence is fresh (see this file's header).
//
// Order matters: sweep first. reportLayoutDrift only reads, but resumeAgents (which runs
// straight after) writes per-pane state, and it must not be writing beside records that
// are about to be deleted.
func afterRestore(save string) {
	if n := hq.ReapDeadPaneStateNow(); n > 0 {
		restoreLogf("afterRestore: dropped %d pane-keyed state file(s) whose panes are gone "+
			"(the server reissues pane ids, so these would have described the panes that inherited their numbers)", n)
	}
	reportLayoutDrift(save)
}
