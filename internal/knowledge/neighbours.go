package knowledge

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Neighbours (hq-knowledge-engine D8): which entries are about the same thing, found
// without a model. Kind first, then keyword overlap on title + body. The measured need
// this serves — on the first mined batch, 11 of 146 leads were one lesson spread over
// five projects, and the expensive part was SEEING that — is met by overlap; embeddings
// are deliberately not used (cgo-free CLI), and would need evidence to be added.
//
// Tokens: lowercase ASCII words of three letters or more, and CJK character bigrams —
// a Chinese sentence has no spaces, and bigrams are the cheapest unit that still carries
// meaning ("回收" survives, "回" alone does not). Stop-words are the handful that carry
// none in either language.

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "not": true, "with": true, "that": true, "this": true,
	"from": true, "when": true, "then": true, "never": true, "always": true, "into": true, "its": true,
	"gtmux": true, "一个": true, "不是": true, "就是": true, "这个": true, "那个": true, "我们": true,
	"可以": true, "没有": true, "的时": true, "时候": true, "什么": true, "应该": true, "一下": true,
	"是不": true, "问题": true, "现在": true, "然后": true, "但是": true, "如果": true, "还是": true,
	// The miner's own vocabulary: every mined error line carries these, so they say
	// nothing about WHICH error it is.
	"error": true, "path": true, "session": true, "sessions": true, "file": true, "line": true,
	"hex": true, "exit": true, "code": true,
}

// minedSuffixRe strips the tally the miner appends to a recurring-error lesson — it is
// the same on every such line.
var minedSuffixRe = regexp.MustCompile(`\s*\(×\d+, \d+ sessions?\)\s*$`)

// tokens returns the set of tokens in s.
func tokens(s string) map[string]bool {
	out := map[string]bool{}
	var word []rune
	flushWord := func() {
		if len(word) >= 3 {
			w := strings.ToLower(string(word))
			if !stopWords[w] {
				out[w] = true
			}
		}
		word = word[:0]
	}
	var prevCJK rune
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flushWord()
			if prevCJK != 0 {
				bg := string([]rune{prevCJK, r})
				if !stopWords[bg] {
					out[bg] = true
				}
			}
			prevCJK = r
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			prevCJK = 0
			word = append(word, r)
		default:
			prevCJK = 0
			flushWord()
		}
	}
	flushWord()
	return out
}

// overlap is the overlap coefficient |A∩B| / min(|A|,|B|). Chosen over Jaccard after
// measuring the design machine's ledger (467 entries, 667 pairs HQ had linked as 同族):
// bodies run 200–600 tokens, so Jaccard punishes a short entry next to a long one and
// left linked pairs at p50 0.07 against random pairs at 0.03 — no usable floor. The
// overlap coefficient put linked pairs at p50 0.17 / p25 0.14 and random pairs at p90
// 0.13, which is a line.
func overlap(a, b map[string]bool) float64 {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if b[t] {
			inter++
		}
	}
	if m < n {
		n = m
	}
	return float64(inter) / float64(n)
}

// shared counts the tokens two sets have in common.
func shared(a, b map[string]bool) int {
	n := 0
	for t := range a {
		if b[t] {
			n++
		}
	}
	return n
}

// entryText is what an entry is compared on: the title counts twice (it is the
// distilled sentence), the body once.
func entryText(op knowledgeOp) string { return op.Title + " " + op.Title + " " + op.Body }

// Neighbour is one match with its score.
type Neighbour struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Score float64 `json:"score"` // 0..1, kind-adjusted
}

// neighbourFloor is the overlap coefficient below which two entries are not about the
// same thing: three quarters of the pairs HQ had linked as 同族 sit above it, nine in ten
// random pairs below (see overlap).
const neighbourFloor = 0.14

// neighboursOf ranks live entries by overlap with text, same kind first. Excludes id.
func neighboursOf(live []knowledgeOp, text, kind, exclude string, n int) []Neighbour {
	probe := tokens(text)
	var out []Neighbour
	for _, op := range live {
		if op.ID == exclude {
			continue
		}
		score := overlap(probe, tokens(entryText(op)))
		if kind != "" && op.Kind == kind {
			score *= 1.25 // same kind: the more likely duplicate
		}
		if score < neighbourFloor {
			continue
		}
		if score > 1 {
			score = 1
		}
		out = append(out, Neighbour{ID: op.ID, Title: op.Title, Kind: op.Kind, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// NeighboursForText is the public read: closest live entries to a piece of text.
func NeighboursForText(text, kind string, n int) ([]Neighbour, error) {
	live, err := liveKnowledge()
	if err != nil {
		return nil, err
	}
	return neighboursOf(live, text, kind, "", n), nil
}

// groupCandidates partitions the pool into families. Each family has a SEED — the
// earliest member — and a candidate joins only if it resembles the seed: transitive
// merging (A like B, B like C, so A with C) glued a real pool of 231 into one blob of
// 51, because "like" is a ratio on short lines and chains of coincidence are cheap.
// Groups keep pool order; singletons stay singletons.
//
// The LESSON only: a mined candidate's context is the tail of an assistant reply, and
// on a real pool those tails share the supervisor's own phrasing across unrelated
// leads. Same topic only, and at least three tokens in common — a ratio alone groups on
// coincidence.
func groupCandidates(cands []Candidate) [][]Candidate {
	n := len(cands)
	toks := make([]map[string]bool, n)
	for i, c := range cands {
		toks[i] = tokens(minedSuffixRe.ReplaceAllString(c.Lesson, ""))
	}
	assigned := make([]bool, n)
	var out [][]Candidate
	for i := 0; i < n; i++ {
		if assigned[i] {
			continue
		}
		group := []Candidate{cands[i]}
		assigned[i] = true
		for j := i + 1; j < n; j++ {
			if assigned[j] || cands[i].Topic != cands[j].Topic {
				continue
			}
			if shared(toks[i], toks[j]) >= candidateGroupMinShared && overlap(toks[i], toks[j]) >= candidateGroupFloor {
				group = append(group, cands[j])
				assigned[j] = true
			}
		}
		out = append(out, group)
	}
	return out
}

// candidateGroupFloor is stricter than the entry floor: a pool line is short, so a
// small overlap is a coincidence more often than a family. Both bounds must hold.
const (
	candidateGroupFloor     = 0.3
	candidateGroupMinShared = 3
)
