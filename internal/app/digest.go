// The `gtmux digest` CLI rendering: the scannable, column-aligned "fleet at a
// glance" report over the radar-assembled digest rows (radar.GatherDigest).
// The digest PRODUCER (the deterministic joins) lives in internal/radar; this
// file is the human-facing table + the command dispatch only.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

// cmdDigest implements `gtmux digest [--json]`.
func cmdDigest(args []string) int {
	jsonOut := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			commandHelp("digest")
			return 0
		default:
			i18n.Sae("gtmux digest: unknown option '"+a+"'", "gtmux digest: 未知选项 '"+a+"'")
			return 2
		}
	}
	if jsonOut {
		b, err := radar.DigestJSONBytes()
		if err != nil {
			i18n.Sae("gtmux: "+err.Error(), "gtmux: "+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	rows := radar.GatherDigest()
	if len(rows) == 0 {
		i18n.Say("No live agents.", "当前没有 agent。")
		return 0
	}
	renderDigestTable(rows)
	return 0
}

// digestSectionSpec is one of the report's fixed sections, in display order.
// needs-you leads (it's the loudest signal); errored is appended only when
// non-empty — it's a modifier on an otherwise-idle row, not a base state.
var digestSectionSpec = []struct{ key, en, zh string }{
	{"needs_input", "needs input", "需要你"},
	{"working", "working", "进行中"},
	{"completed", "completed", "已完成"},
	{"errored", "errored", "出错"},
}

// digestBucket sorts a row into one report section. A row carrying an error
// marker is pulled into "errored" regardless of its underlying status; among
// the rest, waiting leads, idle means done, and "running" (the hookless
// can't-tell-idle-from-working fallback status) folds into "working" — it's
// still an active agent, just a less certain signal — its own grey glyph
// (via statusStyle) keeps that distinction visible on the row itself.
func digestBucket(r radar.DigestRow) string {
	switch {
	case r.Error != "":
		return "errored"
	case r.TaskStatus == radar.TaskStatusUndelivered:
		// A dispatch whose goal never landed leaves a live, EMPTY, idle agent — so by
		// pane status alone it files under "completed", which is the same false signal
		// `gtmux tasks` used to give. It needs you: nothing was ever asked of it.
		return "needs_input"
	case r.Status == "waiting":
		return "needs_input"
	case r.Status == "idle":
		return "completed"
	default:
		return "working"
	}
}

// digestBadge is the row's right-side badge: the dispatch task's lifecycle
// status takes priority (it's the most actionable signal), then how many
// options a waiting prompt offers, then a usage warning or background marker.
func digestBadge(r radar.DigestRow) string {
	switch {
	case r.Task != "" && r.TaskStatus != "":
		return r.TaskStatus
	case r.Ask != "":
		return fmt.Sprintf(i18n.Tr("%d opts", "%d 项"), strings.Count(r.Ask, " · ")+1)
	case r.UsageWarn != "":
		return erroredGlyph
	case r.Bg != "":
		return i18n.Tr("bg", "后台")
	default:
		return ""
	}
}

// digestLabel is a row's name-column identity: its tmux location (or
// "elsewhere" for a sensed native session), tagged for the HQ supervisor row.
func digestLabel(r radar.DigestRow) string {
	name := r.Loc
	if name == "" {
		name = i18n.Tr("elsewhere", "不在 tmux")
	}
	if r.Role == "supervisor" {
		name += " ⌂"
	}
	return name
}

// renderDigestTable prints the formatted, column-aligned status report: a
// one-line summary of counts by state, then a section per state with one
// aligned row per agent — status glyph · name · goal/last (truncated to the
// terminal width) · a right badge · a right-aligned relative time. This is
// gtmux's scannable "fleet at a glance" — no prose paragraphs.
func renderDigestTable(rows []radar.DigestRow) {
	buckets := map[string][]radar.DigestRow{}
	for _, r := range rows {
		k := digestBucket(r)
		buckets[k] = append(buckets[k], r)
	}

	summary := make([]string, 0, 4)
	for _, s := range digestSectionSpec {
		n := len(buckets[s.key])
		if s.key == "errored" && n == 0 {
			continue // an exceptional bucket — only surface it when non-empty
		}
		summary = append(summary, fmt.Sprintf("%d %s", n, i18n.Tr(s.en, s.zh)))
	}
	fmt.Println(i18n.TruncDisp(strings.Join(summary, " · "), termWidth()))

	nameWidth := 8
	for _, r := range rows {
		if w := i18n.DispWidth(digestLabel(r)); w > nameWidth {
			nameWidth = w
		}
	}
	if nameWidth > 24 {
		nameWidth = 24
	}
	tw := termWidth()

	for _, s := range digestSectionSpec {
		rs := buckets[s.key]
		if len(rs) == 0 {
			continue
		}
		heading := i18n.TruncDisp(fmt.Sprintf("%s (%d)", i18n.Tr(s.en, s.zh), len(rs)), tw)
		fmt.Printf("\n%s%s%s\n", i18n.Bold, heading, i18n.Reset)
		for _, r := range rs {
			printDigestTableRow(r, nameWidth, tw)
		}
	}
}

// digestBadgeWidth/digestTimeWidth are fixed column widths for the report's
// two right-hand columns — small and stable enough not to need dynamic sizing.
const (
	digestBadgeWidth = 10
	digestTimeWidth  = 6
)

// fmtAgoShort renders a unix time as a compact "Ns/Nm/Nh/Nd" — tighter than
// devices.go's fmtAgo ("just now" / "Nm ago") so it fits the report's narrow
// right-aligned time column.
func fmtAgoShort(unix int64) string {
	if unix == 0 {
		return "?"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// digestCols is how a row's columns fit a terminal width.
type digestCols struct {
	name, mid   int
	badge, time bool
}

// digestLayout fits a row into tw display columns, counted exactly as
// printDigestTableRow prints them: "  " glyph " " name, then "  " mid, "  " badge,
// "  " time. The middle text gets what is left; under 8 columns it gives way: the time
// column goes first, then the badge, then the name shrinks (to 4), and the middle text
// takes what remains, down to nothing. The budget used to count one space before the
// time where two are printed, and floored the middle at 8, so every row was a column
// too wide and a narrow terminal got rows of 43 columns (%12, 2026-10-06).
func digestLayout(nameWidth, glyphWidth, tw int) digestCols {
	c := digestCols{name: nameWidth, badge: true, time: true}
	used := func() int {
		w := 2 + glyphWidth + 1 + c.name + 2 // the lead, the name, the gap before the middle
		if c.badge {
			w += 2 + digestBadgeWidth
		}
		if c.time {
			w += 2 + digestTimeWidth
		}
		return w
	}
	c.mid = tw - used()
	if c.mid < 8 && c.time {
		c.time = false
		c.mid = tw - used()
	}
	if c.mid < 8 && c.badge {
		c.badge = false
		c.mid = tw - used()
	}
	if c.mid < 8 && c.name > 4 {
		c.name -= 8 - c.mid
		if c.name < 4 {
			c.name = 4
		}
		c.mid = tw - used()
	}
	if c.mid < 0 {
		c.mid = 0
	}
	return c
}

// printDigestTableRow prints one aligned row within its section.
func printDigestTableRow(r radar.DigestRow, nameWidth, tw int) {
	// errored-idle gets its own amber ⚠ marker (never a status color) — same
	// convention as `gtmux agents`, so the glyph itself flags "look here" even
	// outside the errored section's heading.
	glyph, color := erroredGlyph, i18n.Amber
	if r.Error == "" {
		glyph, color, _ = statusStyle(r.Status)
	}
	cols := digestLayout(nameWidth, i18n.DispWidth(glyph), tw)
	name := i18n.TruncDisp(digestLabel(r), cols.name)
	midWidth := cols.mid
	// Priority: an error is why the row is in this section at all, so it wins;
	// then a waiting prompt's ask, the latest reply, the original goal, and
	// finally whatever background/usage text is available — so the middle
	// column is rarely blank even for a headless/no-transcript-yet row.
	mid := r.Error
	for _, cand := range []string{r.Ask, r.Last, r.Goal, r.Bg, r.UsageWarn} {
		if mid != "" {
			break
		}
		mid = cand
	}
	mid = i18n.TruncDisp(mid, midWidth)

	line := fmt.Sprintf("  %s%s%s %s%s%s  %s", color, glyph, i18n.Reset,
		i18n.Bold, i18n.PadRight(name, cols.name), i18n.Reset, i18n.PadRight(mid, midWidth))
	if cols.badge {
		line += "  " + i18n.PadLeft(digestBadge(r), digestBadgeWidth)
	}
	if cols.time {
		line += "  " + i18n.PadLeft(fmtAgoShort(r.Since), digestTimeWidth)
	}
	fmt.Println(line)
}
