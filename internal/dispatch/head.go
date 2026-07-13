package dispatch

import (
	"strings"
	"unicode"
)

// headRunes is how many leading runes of the normalized text form the "head" —
// the fingerprint shared by the UserPromptSubmit event, the draft assert, and the
// fallback history match. ~40 runes is enough to be specific without depending on
// where a TUI re-wraps a long line.
const headRunes = 40

// normalizeSpace collapses all runs of whitespace to a single space and trims —
// but does NOT truncate. It is applied to the delivered text, the event's recorded
// prompt, and any screen region so they compare equal regardless of TUI
// re-wrapping / padding.
func normalizeSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// NormalizeHead returns the first headRunes runes of the space-normalized text —
// the content fingerprint shared by the UserPromptSubmit event, the draft assert,
// and the fallback history match. Exported because the hook records the event head
// with it and Deliver matches against it.
func NormalizeHead(s string) string {
	rs := []rune(normalizeSpace(s))
	if len(rs) > headRunes {
		rs = rs[:headRunes]
	}
	return string(rs)
}

// containsHead reports whether haystack contains the normalized head of needle.
// The haystack is space-normalized but NOT truncated (only the needle is reduced to
// its head), so a fingerprint that would fall past the haystack's own 40-rune cut
// still matches. An empty head never matches (a blank delivery matches nothing).
func containsHead(haystack, needle string) bool {
	head := NormalizeHead(needle)
	if head == "" {
		return false
	}
	return strings.Contains(normalizeSpace(haystack), head)
}

// queuedMarkers are the on-screen indicators that a submitted message was QUEUED
// behind the current turn rather than run immediately (Claude Code's "Press up to
// edit queued messages"). Matched case-insensitively on the normalized capture.
var queuedMarkers = []string{
	"queued message",
	"queued messages",
	"press up to edit queued",
}

// looksQueued reports whether a capture shows a queued-message indicator.
func looksQueued(capture string) bool {
	low := strings.ToLower(capture)
	for _, m := range queuedMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}
