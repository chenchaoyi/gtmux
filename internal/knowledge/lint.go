package knowledge

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Lint (hq-knowledge-engine D8): the ledger audited as a whole, the way every mature
// practice in the survey audits its base (Karpathy's wiki lint, obsidian-mind's vault
// audit). It REPORTS and never edits — the verbs are the only writers — and it runs by
// hand (`gtmux knowledge lint`) and inside self-check, whose brief carries the summary.
//
// Findings, each with the id it is about:
//
//	orphan         a live entry with no [[link]] in or out — knowledge nobody connected
//	broken-link    a [[link]] that names no live entry, topic, or memory-style slug
//	outdated-link  a [[link]] to a superseded entry — resolvable, but stale
//	near-duplicate two live titles that read as the same lesson
//	stale          a hypothesis past its floor; a pending promotion past its floor
//	               (audience everyone exempt); a promotion with no audience
//	assumed-kind   a migrated entry whose kind is the table's guess, not a judgement

// Finding is one lint result.
type Finding struct {
	Check  string `json:"check"`
	ID     string `json:"id"`
	Detail string `json:"detail"`
}

// LintReport is the whole audit with its counts.
type LintReport struct {
	Entries  int            `json:"entries"`
	Findings []Finding      `json:"findings"`
	Counts   map[string]int `json:"counts"`
}

const (
	hypothesisFloorSec = 30 * 24 * 3600
	promotionFloorSec  = 14 * 24 * 3600
	// duplicateFloor is on the whole text, not the title: on the design machine's
	// ledger sibling entries share no title words at all (linked pairs p90 0.09), and
	// two entries more alike than typical siblings (overlap p90 0.26) are the ones to
	// merge or link.
	duplicateFloor = 0.35
)

var linkRe = regexp.MustCompile(`\[\[([^\]\n]{1,120})\]\]`)

// links returns the [[link]] targets in a body that look like references (no spaces,
// no shell characters — `[[ $- == *i* ]]` in a bash snippet is not a link).
func links(body string) []string {
	var out []string
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		t := strings.TrimSpace(m[1])
		if t == "" || strings.ContainsAny(t, " $=*|\"'") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// slugOf is the part of an id after the topic, with the numeric suffix a retitled
// supersede appends stripped: entries link to each other by slug, and a slug that grew
// a `-8` when its title was reworded still names the same lesson.
var numSuffixRe = regexp.MustCompile(`-\d+$`)

func slugOf(id string) string {
	if i := strings.Index(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	return numSuffixRe.ReplaceAllString(id, "")
}

// resolve reports what a link target names: a live entry (its id), a superseded one
// (its live successor's id, outdated=true), a topic, or nothing.
func resolve(target string, live []knowledgeOp, topics map[string]bool, successor map[string]string) (id string, outdated, ok bool) {
	if topics[target] {
		return "", false, true
	}
	want := slugOf(target)
	for _, op := range live {
		if op.ID == target || slugOf(op.ID) == want {
			return op.ID, false, true
		}
	}
	// A dead id: follow the supersede chain to whatever is live now.
	for dead, next := range successor {
		if dead == target || slugOf(dead) == want {
			for hops := 0; hops < 32; hops++ {
				if n, ok := successor[next]; ok {
					next = n
					continue
				}
				break
			}
			for _, op := range live {
				if op.ID == next {
					return op.ID, true, true
				}
			}
		}
	}
	return "", false, false
}

// Lint audits the folded base at now.
func Lint(now int64) (LintReport, error) {
	ops, err := readKnowledgeOps()
	if err != nil {
		return LintReport{}, err
	}
	return lint(ops, now), nil
}

func lint(ops []knowledgeOp, now int64) LintReport {
	live, custom := foldKnowledge(ops), customTopics(ops)
	successor := map[string]string{}
	for _, op := range ops {
		if op.Op == knowledgeOpSupersede && op.Supersedes != "" && op.Supersedes != op.ID {
			successor[op.Supersedes] = op.ID
		}
	}
	rep := LintReport{Entries: len(live), Counts: map[string]int{}}
	add := func(check, id, detail string) {
		rep.Findings = append(rep.Findings, Finding{Check: check, ID: id, Detail: detail})
		rep.Counts[check]++
	}
	topics := map[string]bool{}
	for _, t := range BuiltinTopics {
		topics[t] = true
	}
	for _, t := range custom {
		topics[t.ID] = true
	}
	linked := map[string]bool{} // ids that some entry links TO
	for _, op := range live {
		for _, l := range links(op.Body) {
			id, outdated, ok := resolve(l, live, topics, successor)
			switch {
			case !ok:
				add("broken-link", op.ID, "[["+l+"]] names nothing live")
				continue
			case outdated:
				add("outdated-link", op.ID, "[["+l+"]] was superseded — now "+id)
			}
			if id != "" {
				linked[id] = true
			}
		}
	}
	for _, op := range live {
		if len(links(op.Body)) == 0 && !linked[op.ID] {
			add("orphan", op.ID, "no [[link]] in or out")
		}
		if op.KindAssumed {
			add("assumed-kind", op.ID, "kind "+op.Kind+" is the migration table's guess — `gtmux knowledge kind "+op.ID+" <kind>` to confirm")
		}
		if op.Status == StatusHypothesis && now-op.At >= hypothesisFloorSec {
			add("stale", op.ID, fmt.Sprintf("hypothesis for %dd — confirm, supersede or retire", (now-op.At)/86400))
		}
		if promotionPending(op) {
			switch {
			case op.Audience == "":
				add("stale", op.ID, "promoted with no audience — withdraw, then promote --for")
			case op.Audience != AudienceEveryone && now-op.PromotedAt >= promotionFloorSec:
				add("stale", op.ID, fmt.Sprintf("promotion pending %dd — `gtmux knowledge land %s`", (now-op.PromotedAt)/86400, op.ID))
			}
		}
	}
	// Near-duplicates: pairwise on the whole text, same kind only — a howto and a
	// pitfall with overlapping words are the two sides of one lesson, not a duplicate.
	toks := make([]map[string]bool, len(live))
	for i, op := range live {
		toks[i] = tokens(entryText(op))
	}
	for i := range live {
		for j := i + 1; j < len(live); j++ {
			if live[i].Kind != live[j].Kind {
				continue
			}
			if s := overlap(toks[i], toks[j]); s >= duplicateFloor {
				add("near-duplicate", live[i].ID, fmt.Sprintf("reads like %s (%.2f) — supersede one into the other, or link them", live[j].ID, s))
			}
		}
	}
	sort.SliceStable(rep.Findings, func(a, b int) bool {
		if rep.Findings[a].Check != rep.Findings[b].Check {
			return rep.Findings[a].Check < rep.Findings[b].Check
		}
		return rep.Findings[a].ID < rep.Findings[b].ID
	})
	return rep
}

// Summary is the one-line form for a self-check brief: "lint: 467 entries · 12 orphan ·
// 3 broken-link …", or "" when clean.
func (r LintReport) Summary() string {
	if len(r.Findings) == 0 {
		return ""
	}
	keys := make([]string, 0, len(r.Counts))
	for k := range r.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", r.Counts[k], k))
	}
	return fmt.Sprintf("lint: %d entries · %s", r.Entries, strings.Join(parts, " · "))
}
