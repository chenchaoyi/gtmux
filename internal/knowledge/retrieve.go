package knowledge

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Retrieval: the one answer to "does this text match this entry" (kb-one-retrieval).
//
// There used to be two. `neighbours` tokenized both sides and ranked by overlap;
// the dispatch echo read the rendered topic files line by line and kept a bullet whose
// text CONTAINED a goal word, where the goal was split on spaces. A Chinese goal has no
// spaces, so the whole sentence became one keyword and the substring never matched —
// recall for a Chinese goal was zero, measured. Two implementations also meant two
// different things could be called a match on the same machine.
//
// What ranks, and why weighting was added. Measured against the 904 [[link]] pairs HQ had
// drawn by hand across 720 entries, the unweighted overlap coefficient returned a known
// relative in its top 5 for 33% of them. Every token counted the same there, so `pane`,
// which most of this base carries, weighed as much as `chisel`, which a handful do. At the
// shipped floor about 81 of 719 entries cleared it on any query, and the right one often
// sat below five of those. Weighting each shared token by how rare it is across the live
// base takes the same measurement to 43% at five and 53% at ten.
//
// It is still no model and no index: one pass to count, one pass to score, 60ms over 720
// entries. The 2026-09-12 research said a vector store needs measured evidence first; this
// is the cheap half of that evidence, taken before anything is embedded.

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
func entryText(op knowledgeOp) string {
	return op.Title + " " + op.Title + " " + op.Body + " " + altText(op)
}

// Neighbour is one match with its score.
type Neighbour struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Topic string  `json:"topic,omitempty"`
	Score float64 `json:"score"` // 0..1, kind-adjusted
}

// corpus holds what scoring needs about the live base: each entry's tokens, and how rare
// each token is. Built once per query, which costs one tokenize pass over the base.
type corpus struct {
	live []knowledgeOp
	tok  []map[string]bool
	idf  map[string]float64
}

// newCorpus tokenizes the live entries and counts how many carry each token.
func newCorpus(live []knowledgeOp) *corpus {
	c := &corpus{live: live, tok: make([]map[string]bool, len(live)), idf: map[string]float64{}}
	df := map[string]int{}
	for i, op := range live {
		t := tokens(entryText(op))
		c.tok[i] = t
		for w := range t {
			df[w]++
		}
	}
	n := float64(len(live))
	for w, d := range df {
		// Smoothed inverse document frequency, kept strictly positive. The plain
		// log(N/(1+df)) is 0 or negative on a small base — with two entries sharing a
		// token every weight collapses to zero and nothing matches anything, which is
		// exactly the state a base starts life in. Measured on 720 entries the two forms
		// are within one point of each other (43/53 against 42/52 at five and ten), so
		// the small base is bought cheaply.
		c.idf[w] = math.Log(1 + n/float64(1+d))
	}
	return c
}

// weight is a token's rarity. A token the base has never seen is as rare as one a single
// entry carries: a query word absent from the base should not be free to match on.
func (c *corpus) weight(w string) float64 {
	if v, ok := c.idf[w]; ok {
		return v
	}
	return math.Log(1 + float64(len(c.live))/2)
}

// score is the weighted overlap coefficient between a query's tokens and entry i: the
// rarity the two share, over the rarity of the smaller side. It stays in 0..1 and keeps
// the property the unweighted form was chosen for — a short entry beside a long one is
// not punished for being short.
func (c *corpus) score(probe map[string]bool, i int) float64 {
	a, b := probe, c.tok[i]
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var inter, wa, wb float64
	for w := range a {
		x := c.weight(w)
		wa += x
		if b[w] {
			inter += x
		}
	}
	for w := range b {
		wb += c.weight(w)
	}
	den := wa
	if wb < den {
		den = wb
	}
	if den == 0 {
		return 0
	}
	return inter / den
}

// Query is what a caller is looking for. Text is required; the rest narrow the field.
type Query struct {
	Text    string
	Kind    string   // same kind ranks higher; not a filter
	Topics  []string // when set, only these topics are searched
	Exclude string   // an id to leave out, usually the entry being compared
	N       int      // how many to return; 0 means retrieveDefaultN
}

// retrieveDefaultN is how many matches a caller gets when it does not say. Five was set
// before anything measured how often the right entry sat sixth; at ten the same
// measurement finds half the known relatives instead of a third.
const retrieveDefaultN = 10

// retrieveFloor is the weighted score below which two texts are not about the same thing.
// Measured on the 904 pairs HQ had linked by hand across 720 entries: 83% of them clear
// it, against 6.4% of random pairs, or about 46 of 719 entries on any query. The
// unweighted score it replaces let 11.3% of random pairs through, about 81 per query, at a
// similar rate for the real ones. Callers take the top ten of what clears it, so the floor
// decides when to answer NOTHING, and the ranking decides the rest.
const retrieveFloor = 0.10

// search ranks live entries against a query. It is the only place in this package that
// decides what a match is.
func search(live []knowledgeOp, q Query) []Neighbour {
	if strings.TrimSpace(q.Text) == "" {
		return nil
	}
	if len(q.Topics) > 0 {
		want := map[string]bool{}
		for _, t := range q.Topics {
			want[t] = true
		}
		var keep []knowledgeOp
		for _, op := range live {
			if want[op.Topic] {
				keep = append(keep, op)
			}
		}
		live = keep
	}
	c := newCorpus(live)
	probe := tokens(q.Text)
	var out []Neighbour
	for i, op := range live {
		if op.ID == q.Exclude {
			continue
		}
		s := c.score(probe, i)
		if q.Kind != "" && op.Kind == q.Kind {
			s *= 1.25 // same kind: the more likely duplicate
		}
		if op.Legacy {
			s *= 0.9 // a lesson still awaiting migration yields to a current entry
		}
		if s < retrieveFloor {
			continue
		}
		out = append(out, Neighbour{ID: op.ID, Title: op.Title, Kind: op.Kind, Topic: op.Topic, Score: s})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].ID < out[b].ID
	})
	n := q.N
	if n <= 0 {
		n = retrieveDefaultN
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// neighboursOf ranks live entries near a text, same kind first. Excludes id. It is the
// older entrance to search() and keeps its argument order for the callers that had it.
func neighboursOf(live []knowledgeOp, text, kind, exclude string, n int) []Neighbour {
	return search(live, Query{Text: text, Kind: kind, Exclude: exclude, N: n})
}

// Neighbours ranks the live entries closest to a text, for a caller outside the package.
func Neighbours(text, kind string, n int) ([]Neighbour, error) {
	live, err := liveKnowledge()
	if err != nil {
		return nil, err
	}
	return search(live, Query{Text: text, Kind: kind, N: n}), nil
}

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
