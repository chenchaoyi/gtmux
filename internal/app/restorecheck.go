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
// but it ends the silence: the drift is named in the log store and on the terminal at the
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
	Layout  string   // normalized (see normalizeLayout) — comparable across a restore
	PaneIDs []string // layout leaf order, used only for the read-only receipt
}

// key identifies the window across the save/live boundary.
func (w windowShape) key() string { return w.Session + ":" + w.Index }

// layoutChecksumRe matches the 4-hex checksum tmux prefixes onto a layout string, which
// is also how a layout FIELD is recognized positionally-independently.
var layoutChecksumRe = regexp.MustCompile(`^[0-9a-f]{4},\d+x\d+,\d+,\d+`)

// layoutLeafRe matches a leaf cell's trailing pane number — `WxH,X,Y,<pane>`. A CONTAINER
// cell is `WxH,X,Y` followed by `{` or `[`, so it never matches and its geometry survives.
var layoutLeafRe = regexp.MustCompile(`(\d+x\d+,\d+,\d+),(\d+)`)

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
					w.PaneIDs = layoutPaneIDs(fld)
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
		out = append(out, windowShape{Session: f[0], Index: f[1], Panes: n, Layout: normalizeLayout(f[3]), PaneIDs: layoutPaneIDs(f[3])})
	}
	return out
}

// layoutDifference keeps the full evidence for logs and a short terminal diagnosis.
type layoutDifference struct {
	saved, live windowShape
	missing     bool
}

func (d layoutDifference) detail() string {
	s, l := d.saved, d.live
	switch {
	case d.missing:
		return i18n.Tr(fmt.Sprintf("%s: window did not come back", s.key()), fmt.Sprintf("%s: 窗口未恢复", s.key()))
	case s.Panes != l.Panes:
		return i18n.Tr(
			fmt.Sprintf("%s: saved with %d pane(s), now has %d. tmux refuses to apply a layout to a different pane count; inspect the window before changing or closing panes", s.key(), s.Panes, l.Panes),
			fmt.Sprintf("%s: 备份中有 %d 个窗格，现在有 %d 个。数量不同时无法直接套用备份布局，请先检查窗口，再调整或关闭窗格", s.key(), s.Panes, l.Panes))
	default:
		return i18n.Tr(
			fmt.Sprintf("%s: arrangement differs: saved %s, now %s", s.key(), s.Layout, l.Layout),
			fmt.Sprintf("%s: 排布和存档不一样：存档 %s，现在 %s", s.key(), s.Layout, l.Layout))
	}
}

// layoutDrift compares arrangements, preferring pane-count mismatch over geometry.
// Newly created live windows are deliberately not drift; missing saved windows are.
func layoutDrift(saved, live []windowShape) []layoutDifference {
	byKey := map[string]windowShape{}
	for _, w := range live {
		byKey[w.key()] = w
	}
	var out []layoutDifference
	for _, s := range saved {
		l, ok := byKey[s.key()]
		if !ok {
			out = append(out, layoutDifference{saved: s, missing: true})
		} else if s.Panes != l.Panes || s.Layout != "" && s.Layout != l.Layout {
			out = append(out, layoutDifference{saved: s, live: l})
		}
	}
	return out
}

// All differences go to the log; only a bounded head appears in the terminal.
const driftReportMax = 5

func reportLayoutDrift(save string) {
	if save == "" {
		return
	}
	saved := savedShapes(save)
	if len(saved) == 0 {
		restoreLogf("restore.layout", "layout-check: save has no window lines — nothing to reconcile")
		return
	}
	live := liveShapes()
	if len(live) == 0 {
		restoreLogf("restore.layout", "layout-check: no live windows to compare against — skipped")
		return
	}
	fmt.Print(renderRestorePaneIDs(saved, live, termWidth()))
	drift := layoutDrift(saved, live)
	if len(drift) == 0 {
		restoreLogf("restore.layout", "layout-check: %d saved window(s) all came back with the same panes and arrangement", len(saved))
		return
	}
	// One record per window prevents a large restore from truncating the tail at
	// the diagnostic store's per-entry limit. The terminal still gets only a head.
	restoreLogf("restore.layout", "layout-check: %d of %d saved window(s) came back DIFFERENT", len(drift), len(saved))
	for _, d := range drift {
		restoreLogf("restore.layout.window", "%s", d.detail())
	}
	fmt.Fprint(os.Stderr, renderLayoutDrift(len(saved), drift, termWidth()))
}

func renderLayoutDrift(total int, drift []layoutDifference, width int) string {
	if len(drift) == 0 {
		return ""
	}
	width = maxInt(1, min(width, 100))
	var b strings.Builder
	line := func(s string) { b.WriteString(i18n.TruncDisp(s, width) + "\n") }
	paragraph := func(s string) {
		for _, part := range i18n.WrapDisp(s, maxInt(1, width-4)) {
			b.WriteString("    " + part + "\n")
		}
	}
	b.WriteByte('\n')
	// Spell out both totals: a bare "2/21" was mistaken for a date (2026-09-24).
	line(i18n.Tr(fmt.Sprintf("⚠ Layout differs in %d of %d saved windows", len(drift), total),
		fmt.Sprintf("⚠ %d 个窗口的布局与备份不同（共 %d 个窗口）", len(drift), total)))
	for i, d := range drift {
		if i == driftReportMax {
			line(i18n.Tr(fmt.Sprintf("  … %d more windows", len(drift)-driftReportMax),
				fmt.Sprintf("  …… 另有 %d 个窗口", len(drift)-driftReportMax)))
			break
		}
		fact := i18n.Tr("Window missing. Check the restore log to find out why it was not created.", "窗口未恢复。请查看恢复日志，确认未能创建的原因。")
		if !d.missing {
			fact = i18n.Tr("Pane sizes or positions differ. Keep the current layout if it suits you, or adjust it in the terminal.", "窗格大小或位置不同。现在的布局合适就可保留，也可以在终端手动调整。")
			if d.saved.Panes != d.live.Panes {
				fact = i18n.Tr(fmt.Sprintf("Panes: saved %d, now %d.", d.saved.Panes, d.live.Panes),
					fmt.Sprintf("窗格：备份 %d 个，现在 %d 个。", d.saved.Panes, d.live.Panes))
				if d.live.Panes > d.saved.Panes {
					fact += i18n.Tr(fmt.Sprintf(" %d extra.\nCheck which panes you still need before closing any.", d.live.Panes-d.saved.Panes),
						fmt.Sprintf("多了 %d 个。\n请先核对哪些仍在使用，确认不需要后再关闭。", d.live.Panes-d.saved.Panes))
				} else {
					fact += i18n.Tr(fmt.Sprintf(" %d missing.\nCheck the restore log before changing the backup.", d.saved.Panes-d.live.Panes),
						fmt.Sprintf("少了 %d 个。\n请先查看恢复日志，保留这份备份。", d.saved.Panes-d.live.Panes))
				}
			}
		}
		line("  " + restoreInline(d.saved.key()))
		paragraph(fact)
		if len(d.live.PaneIDs) > 0 {
			b.WriteString(i18n.Tr("    Open this window: ", "    查看窗口：") + "gtmux focus " + d.live.PaneIDs[0] + "\n")
		}
	}
	// Keep the command intact and copyable even on a narrow terminal.
	b.WriteString(i18n.Tr("Full details:", "完整明细：") + "\n  " + restoreTraceHint + "\n\n")
	return b.String()
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
		fmt.Sprintf("Using the layout backup saved at %s (%s ago).\nChanges made after that backup will not be restored.", when, shortAge(age)),
		fmt.Sprintf("本次使用 %s 的布局备份（%s保存）。\n保存之后的改动不会恢复。", when, restoreAgeChinese(age)))
}

func restoreAgeChinese(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "不到 1 分钟前"
	case age < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(age.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(age.Hours()/24))
	}
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
		restoreLogf("restore.after", "afterRestore: dropped %d pane-keyed state file(s) whose panes are gone "+
			"(the server reissues pane ids, so these would have described the panes that inherited their numbers)", n)
	}
	reportLayoutDrift(save)
}
