package hq

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// pend builds a ledger entry already on the plate.
func pend(id, pane, tier, goal string, since int64) dispatch.Task {
	return dispatch.Task{
		ID: id, Pane: pane, Goal: goal, Tier: tier, CreatedAt: since,
		FirstSeen: since, AwaitingSince: since,
		Disposition: dispatch.DispositionAwaitingCommander,
	}
}

// The view's order is TOTAL and by weight: decision grade first, then the oldest wait,
// then pane, then id. A view that is re-read constantly must not wobble.
func TestPendingOrderIsStableAndByGrade(t *testing.T) {
	// Deliberately fed in an order that contradicts every rank.
	in := []dispatch.Task{
		pend("t-e", "%5", "quiet", "ledger-grade, oldest of all", 100),
		pend("t-b", "%2", "normal", "attention, newer", 500),
		pend("t-a", "%1", "normal", "attention, older", 300),
		pend("t-d", "%4", "critical", "decision, newer", 900),
		pend("t-c", "%3", "critical", "decision, older", 200),
	}
	rows := pendingTasks(in, 1000)
	var got []string
	for _, r := range rows {
		got = append(got, r.row.ID)
	}
	want := []string{"t-c", "t-d", "t-a", "t-b", "t-e"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}

	// Same-grade, same-wait entries fall back to pane then id, so ties cannot flip.
	tie := []dispatch.Task{
		pend("t-z", "%9", "normal", "second", 42),
		pend("t-y", "%9", "normal", "first", 42),
		pend("t-x", "%1", "normal", "pane wins over id", 42),
	}
	rows = pendingTasks(tie, 1000)
	got = nil
	for _, r := range rows {
		got = append(got, r.row.ID)
	}
	if strings.Join(got, ",") != "t-x,t-y,t-z" {
		t.Fatalf("tiebreak order = %v", got)
	}
}

// Two reads of an unchanged set must be byte-identical — the whole point of a standing
// view is that it stops churning. A relative clock in the output would break this.
func TestPendingRenderIsByteIdenticalAcrossReads(t *testing.T) {
	in := []dispatch.Task{
		pend("t-a", "%1", "critical", "ship v0.48.0 or hold?", 1700000000),
		pend("t-b", "%2", "normal", "which branch for the migration?", 1700000100),
	}
	var first, second bytes.Buffer
	renderPending(&first, pendingTasks(in, 2_000_000_000), false)
	renderPending(&second, pendingTasks(in, 2_000_009_999), false) // a later "now"
	if first.String() != second.String() {
		t.Fatalf("the view moved with the clock:\n%q\n%q", first.String(), second.String())
	}
	if strings.Count(first.String(), "\n") != 2 {
		t.Fatalf("expected one line per pending entry, got:\n%s", first.String())
	}
	// The grade is carried by the glyph, so a colourless read loses nothing.
	if !strings.Contains(first.String(), "◆") || !strings.Contains(first.String(), "▸") {
		t.Errorf("grade glyphs missing from the view:\n%s", first.String())
	}
	if strings.Contains(first.String(), "\033[") {
		t.Errorf("colour leaked into a colour-off render:\n%q", first.String())
	}
	var painted bytes.Buffer
	renderPending(&painted, pendingTasks(in, 2_000_000_000), true)
	if !strings.Contains(painted.String(), "\033[") {
		t.Errorf("colour-on render carries no escape: %q", painted.String())
	}
	if stripANSI(painted.String()) != first.String() {
		t.Errorf("colour changed the text, not just its colour:\n%q\n%q", painted.String(), first.String())
	}
}

// Entering and leaving the pending set — the two transitions the view exists to show.
func TestPendingSetEntryAndExit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := dispatch.NewID(1000)
	if err := dispatch.AddTask(dispatch.Task{ID: id, Pane: "%3", Goal: "decide", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if rows := pendingTasks(dispatch.ListTasks(), 100); len(rows) != 0 {
		t.Fatalf("a plain dispatch is not on the plate, got %d rows", len(rows))
	}

	if !dispatch.MarkAwaitingCommander(id, 200) {
		t.Fatal("mark failed")
	}
	rows := pendingTasks(dispatch.ListTasks(), 300)
	if len(rows) != 1 || rows[0].row.ID != id || rows[0].since != 200 {
		t.Fatalf("entry did not join the plate: %+v", rows)
	}
	if rows[0].row.Status != "pending" {
		t.Errorf("row status = %q, want pending", rows[0].row.Status)
	}

	if !dispatch.ClearAwaitingCommander(id, "decided", 400) {
		t.Fatal("clear failed")
	}
	if rows = pendingTasks(dispatch.ListTasks(), 500); len(rows) != 0 {
		t.Fatalf("entry did not leave the plate: %+v", rows)
	}

	// Archiving is closure: a still-marked entry that was archived is off the plate.
	if !dispatch.MarkAwaitingCommander(id, 600) {
		t.Fatal("re-mark failed")
	}
	if !dispatch.ArchiveTask(id, 700) {
		t.Fatal("archive failed")
	}
	if rows = pendingTasks(dispatch.ListTasks(), 800); len(rows) != 0 {
		t.Fatalf("an archived entry is still on the plate: %+v", rows)
	}
}

// A ledger entry from before this change — no tier, no awaiting_since, the disposition
// set by hand — still renders: it reads as attention grade and falls back to its
// created-at stamp rather than dropping out of the view.
func TestPendingLegacyEntryRenders(t *testing.T) {
	legacy := dispatch.Task{
		ID: "t-legacy", Pane: "%7", Goal: "an old escalation", CreatedAt: 1700000000,
		Disposition: dispatch.DispositionAwaitingCommander,
	}
	rows := pendingTasks([]dispatch.Task{legacy}, 1700009999)
	if len(rows) != 1 {
		t.Fatalf("legacy pending entry dropped: %+v", rows)
	}
	if rows[0].since != 1700000000 {
		t.Errorf("wait clock = %d, want the created-at fallback", rows[0].since)
	}
	var buf bytes.Buffer
	renderPending(&buf, rows, false)
	if !strings.Contains(buf.String(), "▸") || !strings.Contains(buf.String(), "an old escalation") {
		t.Errorf("legacy row rendered wrong:\n%s", buf.String())
	}
}

// en + zh, per the i18n rule — including the empty case, which is what the view shows
// most of the time and is the line a brief points at.
func TestPendingEmptyIsBilingual(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })

	i18n.SetLang("en")
	var en bytes.Buffer
	renderPending(&en, nil, false)
	if !strings.Contains(en.String(), "Nothing is awaiting your decision.") {
		t.Errorf("en empty view = %q", en.String())
	}

	i18n.SetLang("zh")
	var zh bytes.Buffer
	renderPending(&zh, nil, false)
	if !strings.Contains(zh.String(), "没有待你决定的事项。") {
		t.Errorf("zh empty view = %q", zh.String())
	}
	// The rows themselves are data (id / pane / goal), so only the chrome translates —
	// but the two languages must not render the same string by accident.
	if en.String() == zh.String() {
		t.Error("the empty view is not translated")
	}
}

// The command surface: marking round-trips through `gtmux tasks --await/--resolve`, an
// unknown id fails loudly rather than silently doing nothing, and a flag missing its
// value is a usage error (never a mark of the empty id).
func TestCmdTasksPlateMutations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := dispatch.NewID(3000)
	if err := dispatch.AddTask(dispatch.Task{ID: id, Pane: "%4", Goal: "decide", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if code := CmdTasks([]string{"--await", id}); code != 0 {
		t.Fatalf("--await exit = %d", code)
	}
	if rows := pendingTasks(dispatch.ListTasks(), 100); len(rows) != 1 {
		t.Fatalf("--await did not reach the ledger: %+v", rows)
	}
	if code := CmdTasks([]string{"--resolve", id, "decided"}); code != 0 {
		t.Fatalf("--resolve exit = %d", code)
	}
	got, _ := dispatch.LoadTask(id)
	if got.AwaitingCommander() || got.Disposition != "decided" {
		t.Fatalf("--resolve left %+v", got)
	}
	if code := CmdTasks([]string{"--await", "t-nope"}); code != 1 {
		t.Errorf("unknown id exit = %d, want 1", code)
	}
	if code := CmdTasks([]string{"--await"}); code != 2 {
		t.Errorf("missing value exit = %d, want 2", code)
	}
	// A following FLAG is not a disposition.
	if code := CmdTasks([]string{"--resolve", id, "--json"}); code != 0 {
		t.Fatalf("--resolve … --json exit = %d", code)
	}
	if got, _ = dispatch.LoadTask(id); got.Disposition != "" {
		t.Errorf("a flag was swallowed as a disposition: %q", got.Disposition)
	}
}

// stripANSI removes SGR sequences so a painted render can be compared to a plain one.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
