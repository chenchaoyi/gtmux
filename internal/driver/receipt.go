package driver

import (
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

// eventsReceipt is the shared Receipt implementation for every hook-equipped
// agent: the gtmux hook records each prompt submission as a UserPromptSubmit on
// the session-events stream, its Summary produced by the SAME pipeline
// (dispatch.NormalizeNeedle) the caller derives the needle with — so the receipt
// check is agent-agnostic: scan the pane's events since the delivery started for
// a matching head. Event density varies by agent (a Codex session emits fewer
// hook occasions than a Claude one); sparsity only lowers the hit rate, never
// the correctness — no match is NoEvidence and the judgment falls to Layer 1.
//
// The stream cannot see a draft, so this implementation never returns
// Unsubmitted: that verdict belongs to evidence sources that can observe a
// paste sitting unsubmitted (the wake channel's draft check, P2).
func eventsReceipt(pane, needle string, since int64) Verdict {
	now := time.Now().Unix()
	win := now - since + 2
	if win < 1 {
		win = 1
	}
	for _, r := range events.Read(win, now) {
		if r.Pane != pane || r.Ts < since || r.Event != "UserPromptSubmit" {
			continue
		}
		if dispatch.HeadsMatch(r.Summary, needle) {
			return Confirmed
		}
	}
	return NoEvidence
}
