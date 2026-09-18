package i18n

import "strings"

// Nearest returns up to n candidates closest to what was typed, best first, or nil
// when nothing is close. It is for the line after "no such command": naming the two
// or three plausible ones turns a refusal into an answer, where the full list is
// something to read through.
//
// The rules are the mistakes people and agents actually make: a prefix ("kn" for
// "knowledge"), a word that contains it or is contained by it ("search" against
// "research"), and a typo within an edit or two ("carriers" for "carrier").
func Nearest(typed string, options []string, n int) []string {
	typed = strings.ToLower(strings.TrimSpace(typed))
	if typed == "" || n <= 0 {
		return nil
	}
	type scored struct {
		name string
		rank int // lower is closer
	}
	var hits []scored
	for _, o := range options {
		lo := strings.ToLower(o)
		switch {
		case lo == typed:
			hits = append(hits, scored{o, 0})
		case strings.HasPrefix(lo, typed):
			hits = append(hits, scored{o, 1})
		case strings.Contains(lo, typed) || strings.Contains(typed, lo):
			hits = append(hits, scored{o, 2})
		default:
			if d := editDistance(typed, lo); d <= maxEdits(typed) {
				hits = append(hits, scored{o, 2 + d})
			}
		}
	}
	// Insertion sort: the option lists here are tens of entries, and a stable order
	// keeps the suggestion the same every time the same thing is mistyped.
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].rank < hits[j-1].rank; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	var out []string
	for _, h := range hits {
		if len(out) == n {
			break
		}
		out = append(out, h.name)
	}
	return out
}

// maxEdits scales with the word: one edit in a short word is most of it, while a long
// word survives two without becoming a different word.
func maxEdits(s string) int {
	switch n := len([]rune(s)); {
	case n <= 3:
		return 1
	case n <= 7:
		return 2
	default:
		return 3
	}
}

// editDistance is Levenshtein over runes, two rows at a time.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = minOf(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func minOf(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
