package app

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

// Every line of the digest table fits the terminal (the agent-digest spec's "fit the
// terminal width"). The width budget counted one space before the time column where two
// are printed, and floored the middle column at 8, so rows were a column too wide at 80
// and 100 and 43 columns wide on a 20-column terminal (%12, 2026-10-06; en and zh).
func TestDigestTableFitsTheTerminal(t *testing.T) {
	now := time.Now().Unix()
	rows := []radar.DigestRow{
		{Agent: "claude", Loc: "a-rather-long-session-name:0.1", Status: "waiting", Ask: strings.Repeat("Do you want to proceed with the migration? ", 4), Since: now - 30},
		{Agent: "codex", Loc: "work:1.0", Status: "working", Last: strings.Repeat("正在重写测试并检查中文显示宽度 ", 4), Since: now - 3600},
		{Agent: "claude", Loc: "idle-one:2.0", Status: "idle", Error: "rate limited by the provider", Since: now - 86400*3},
		// The widest badge there is (%12's edge case on 432ba16e: 81 wide at 80).
		{Agent: "codex", Loc: "spawned:3.0", Status: "working", Task: "ship it", TaskStatus: "undelivered", Last: strings.Repeat("x", 120), Since: now - 60},
	}
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) }) // SetLang("") is a no-op: restore what was set
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		for _, tw := range []int{6, 8, 10, 12, 20, 30, 39, 40, 52, 80, 100, 160} {
			t.Setenv("COLUMNS", strconv.Itoa(tw))
			out := captureStdout(t, func() { renderDigestTable(rows) })
			for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if w := i18n.DispWidth(stripANSI(line)); w > tw {
					t.Errorf("%s at %d columns: a line is %d wide: %q", lang, tw, w, stripANSI(line))
				}
			}
		}
	}
}

// What gives way, and in which order, when the middle text runs short.
func TestDigestLayoutGivesWayInOrder(t *testing.T) {
	wide := digestLayout(12, 1, 100)
	if !wide.badge || !wide.time || wide.name != 12 || wide.mid != 100-(2+1+1+12+2+2+digestBadgeWidth+2+digestTimeWidth) {
		t.Fatalf("100 columns: %+v", wide)
	}
	if c := digestLayout(12, 1, 40); c.time || !c.badge || c.mid < 8 {
		t.Fatalf("40 columns drops the time first: %+v", c)
	}
	if c := digestLayout(12, 1, 30); c.time || c.badge || c.mid < 8 {
		t.Fatalf("30 columns drops the badge next: %+v", c)
	}
	if c := digestLayout(12, 1, 16); c.name != 4 || c.mid != 16-(2+1+1+4+2) {
		t.Fatalf("16 columns shrinks the name to 4: %+v", c)
	}
	if c := digestLayout(12, 1, 8); c.midOn || c.name != 4 || c.badge || c.time {
		t.Fatalf("8 columns: glyph and name only: %+v", c)
	}
}
