package events

import (
	"strings"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// Who wrote a prompt, worked out at READ time (issue #1156).
//
// Two `UserPromptSubmit` records on the same pane are field-for-field identical whether
// the commander typed the words or the supervisor delivered them: same event, same
// `origin:"instruction"`, same severity, same `agent_session`. The only thing that tells
// them apart is the `gtmux:audit:send` that follows a delivery — and the supervisor's pull
// view withholds the audit trail, so the distinction was invisible unless the reader
// remembered to add `--all`.
//
// That is a trap rather than a discipline, and it fails in the dangerous direction. Taking
// the commander's words for your own costs one repeated action; taking your OWN words for
// the commander's is self-authorization, which is the failure the charter's self-rotate
// section was written about.
//
// So the join happens here, on the way out, and the reader is simply told. Nothing about
// what is owed changes: the audit trail is still excluded from the consumption debt and
// still hidden from the default view. It is read for the ANSWER and not for display.

// PayloadHead normalizes text to the key both sides of the join compare on: whitespace
// collapsed, first headRunes runes. The audit trail records a bounded head of a payload
// and the event records a bounded head of a prompt, so a shared fold is what makes them
// comparable at all.
func PayloadHead(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > headRunes {
		r := []rune(s)
		s = string(r[:headRunes])
	}
	return s
}

// headRunes is the compare width. An audit summary is budgeted at 200 bytes for a send,
// so a shorter head is what both sides can be trusted to share.
const headRunes = 60

// minHeadForPrefix is how much recorded text a PREFIX match needs before it may claim a
// prompt. A long instruction is recognised by its first runes even when the two sides kept
// different amounts of it; a short one has no such margin, and "继续" begins any number of
// unrelated prompts, so below this width the two have to agree exactly.
const minHeadForPrefix = 12

// AuthorOf answers, for each prompt submission in recs, WHO put those words in the pane.
// The result is keyed by sequence number; an absent entry means nobody but the person at
// the keyboard, which is the answer for most prompts and the safe default for all of them.
//
// It reads the whole slice, including records a caller intends to hide: the audit trail is
// the evidence, so withholding it from the join would be withholding the answer.
func AuthorOf(recs []Record) map[int64]string {
	// pane → the heads delivered into it, and by whom.
	byPane := map[string]map[string]string{}
	for _, r := range recs {
		if r.Event != AuditEventSend || r.Pane == "" || r.Actor == "" {
			continue
		}
		state, payload, ok := strings.Cut(r.Summary, ": ")
		// A refused or failed delivery never reached the pane, so its words cannot be the
		// prompt that arrived, and crediting it would put another party's name on the
		// commander's own line.
		if !ok || Outcome(state) != diag.OK {
			continue
		}
		if h := PayloadHead(payload); h != "" {
			if byPane[r.Pane] == nil {
				byPane[r.Pane] = map[string]string{}
			}
			byPane[r.Pane][h] = r.Actor
		}
	}
	if len(byPane) == 0 {
		return nil
	}
	out := map[int64]string{}
	for _, r := range recs {
		if r.Event != "UserPromptSubmit" || r.Pane == "" || r.Seq == 0 {
			continue
		}
		if who := MatchHead(byPane[r.Pane], r.Summary); who != "" {
			out[r.Seq] = who
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MatchHead finds who delivered this text, or "" for nobody.
//
// The two sides are not the same string and cannot be compared as though they were: EACH
// is independently truncated by whoever wrote it, and the harness can append its own
// blocks to the prompt it reports. So the test is that one is a prefix of the other,
// whichever way round, and the longest such agreement wins because it is the more
// specific claim.
//
// Measured on the real journal: the hook budgets a prompt's head shorter than the audit
// trail budgets a payload's, so the delivery's record is the LONGER of the two. A test in
// one direction only silently attributed nothing at all.
func MatchHead(heads map[string]string, text string) string {
	if len(heads) == 0 {
		return ""
	}
	folded := PayloadHead(text)
	if folded == "" {
		return ""
	}
	if who, ok := heads[folded]; ok {
		return who
	}
	best, who := 0, ""
	for head, actor := range heads {
		n := sharedHead(folded, head)
		if n < minHeadForPrefix || n <= best {
			continue
		}
		best, who = n, actor
	}
	return who
}

// sharedHead is how much the two agree on when one begins the other, in runes, and 0 when
// neither does. It is deliberately not a longest-common-prefix: two unrelated lines that
// happen to open the same way are not the same line, and a partial agreement is exactly
// the kind of near-miss that would put the wrong name on the commander's words.
func sharedHead(a, b string) int {
	switch {
	case strings.HasPrefix(a, b):
		return utf8.RuneCountInString(b)
	case strings.HasPrefix(b, a):
		return utf8.RuneCountInString(a)
	default:
		return 0
	}
}
