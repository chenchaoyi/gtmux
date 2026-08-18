package radar

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// claudeResumeMenu is Claude's reopened-session picker as it is actually drawn: a
// numbered menu inside the same box chrome an input composer uses.
const claudeResumeMenu = `╭────────────────────────────────────────────╮
│ Continue this conversation?                │
│                                            │
│ ❯ 1. Resume from summary                   │
│   2. Resume full session                   │
│   3. Start fresh                           │
╰────────────────────────────────────────────╯`

// THE 2026-08-18 FALSE ALARM, in its own terms. gtmux screen-scraped a pane that was
// sitting at Claude's resume menu, found "text in the box", and announced the pane was
// stuck holding an undelivered goal. It is chrome the agent PAINTED; nobody typed it.
func TestClassifyStuck_ResumeMenuIsNotADraft(t *testing.T) {
	if got := classifyStuck(claudeResumeMenu, "", true); got != "" {
		t.Fatalf("a resume menu classified as %q; want \"\" — the menu is the agent's own chrome, not a user draft", got)
	}
	// Same through the display label the HQ slow tick carries, not just the registry key.
	if got := classifyStuck(claudeResumeMenu, "Claude Code", true); got != "" {
		t.Errorf("resume menu under the display label classified as %q; want \"\"", got)
	}
}

// And it must not swing the other way either: a picker is NOT a needs-you block. Calling
// it "startup" would knock HQ about every reopened session sitting at its menu — the
// original false-positive the picker list exists to prevent, in a new costume.
func TestClassifyStuck_ResumeMenuIsNotAStartupGateEither(t *testing.T) {
	if got := classifyStuck(claudeResumeMenu, "", true); got == "startup" {
		t.Fatal("a resume picker must not read as a startup GATE — a reopened session parked at its menu is not blocked work")
	}
}

// The narrowing that keeps this from becoming the NEXT false positive: a pane that merely
// MENTIONS the menu's wording — a worker reading this very file, say — is not sitting at
// one. Same defect shape as the quoted-gate-phrase test next door.
func TestClassifyStuck_ResumeWordingInScrollbackIsNotAMenu(t *testing.T) {
	var b strings.Builder
	b.WriteString("looking at internal/prompt/prompt.go:\n")
	b.WriteString("  \"Resume from summary\",       // Claude resume picker\n")
	b.WriteString("  \"Resume full session\",       // Claude resume picker\n")
	for i := 0; i < 30; i++ {
		b.WriteString("  ok\n")
	}
	b.WriteString("╭──────────────────────────────╮\n│ ❯ finish the migration       │\n╰──────────────────────────────╯")

	if got := classifyStuck(b.String(), "", true); got != "draft" {
		t.Fatalf("classifyStuck = %q; want \"draft\" — the menu wording was up in scrollback, and the box really does hold a typed draft", got)
	}
}

// The other half of the same incident: the reason gtmux looked at that pane at all was a
// dispatch record created two weeks earlier, for a session long gone, still carrying
// `delivered:false` and still naming a pane id the reboot had handed to someone else.
func TestStaleUndeliveredDispatchStopsSteeringItsPane(t *testing.T) {
	now := time.Now().Unix()
	fresh := dispatch.Task{ID: "t1", Pane: "%25", CreatedAt: now - 3600}
	old := dispatch.Task{ID: "t2", Pane: "%25", CreatedAt: now - 15*24*3600}

	if fresh.StaleUndelivered(now) {
		t.Error("an hour-old undelivered dispatch is live work — it must still drive the stuck check")
	}
	if !old.StaleUndelivered(now) {
		t.Error("a 15-day-old undelivered dispatch has outlived the pane numbering it is keyed to")
	}
	// The fact itself does not expire — only its authority over a pane id does.
	if !old.Undelivered() {
		t.Error("the ledger must keep saying the goal never landed; `gtmux tasks` reports the truth")
	}
	// A dispatch that LANDED is never stale-undelivered, however old.
	landed := dispatch.Task{ID: "t3", Pane: "%25", CreatedAt: now - 90*24*3600, Delivered: true}
	if landed.StaleUndelivered(now) {
		t.Error("a delivered dispatch is finished work, not an expired one")
	}
}
