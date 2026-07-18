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

// ContainsHead reports whether haystack contains the normalized head of needle.
// The haystack is space-normalized but NOT truncated (only the needle is reduced to
// its head), so a fingerprint that would fall past the haystack's own 40-rune cut
// still matches. An empty head never matches (a blank delivery matches nothing).
//
// Exported because the HQ wake channel acks its own deliveries with it (hqnudge
// matches the batch id against the pane capture) — one definition of "did this text
// reach the screen", shared with Deliver's fallback.
func ContainsHead(haystack, needle string) bool {
	head := NormalizeHead(needle)
	if head == "" {
		return false
	}
	return strings.Contains(normalizeSpace(haystack), head)
}

// NormalizeTail returns the LAST headRunes runes of the space-normalized text — the
// TRAILING fingerprint, mirror of NormalizeHead. Pairing head+tail is what lets the
// pre-submit check tell a FULLY-rendered draft from a half-rendered one whose head
// arrived a frame before its tail: a head-only match would submit the payload
// truncated. For text no longer than headRunes, tail == head (the whole thing).
func NormalizeTail(s string) string {
	rs := []rune(normalizeSpace(s))
	if len(rs) > headRunes {
		rs = rs[len(rs)-headRunes:]
	}
	return string(rs)
}

// ContainsTail reports whether haystack contains the normalized tail of needle —
// the counterpart to ContainsHead. Used together they assert the draft holds the
// WHOLE delivery (head AND tail), not just its leading edge.
func ContainsTail(haystack, needle string) bool {
	tail := NormalizeTail(needle)
	if tail == "" {
		return false
	}
	return strings.Contains(normalizeSpace(haystack), tail)
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
