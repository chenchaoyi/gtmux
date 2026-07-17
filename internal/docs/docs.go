// Package docs keeps the documentation's factual claims honest about the code.
//
// The problem it exists for: a doc that shows a format the code cannot produce is a
// TESTABLE FALSEHOOD, and for four incidents running we tested nothing. docs/cli.md
// described the wake channel as it worked before hq-perception-v2 — a dead format, a
// `done` trigger long since generalized, 8 of 12 classes missing — and a manual docs
// audit two days earlier had walked straight past it (#473 → #478). `attach` shipped
// undocumented. DESIGN.md still names Go files the Swift migration deleted, drift that
// earned a compatibility table instead of a fix.
//
// So: a documented example of a code-generated format is marked with its registry id,
// and this package renders that id from the REAL builder and compares. CI reports;
// `make docs-fix` rewrites. Nobody hand-transcribes a line again.
//
// The boundary, stated where it can't be missed: this checks claims that have a
// machine-readable source — a rendered format, an enumeration's membership. It does NOT
// check whether prose is true. "The done wake fires for any session" is a sentence about
// behavior; no fixture catches it being wrong. That stays a reviewer's judgment, and a
// green run here is not a reviewed doc.
package docs

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

// Examples maps a doc region id → the line(s) the code actually produces for it.
//
// The registry is Go on purpose: it cannot compile against a builder whose shape
// changed, so the doc borrows one more link of the chain the code already has. Naming
// the region also resolves what a bare rendered line cannot — WHICH call produced it
// (class? head? which fields?) — and the id is what a reviewer greps when they touch the
// builder.
var Examples = map[string]func() string{
	// docs/cli.md — "The wake channel" — the two shapes a reader meets first: a knock
	// that needs them, and a completion they can judge from the line alone.
	"wake-lines": func() string {
		return strings.Join([]string{
			hqwake.Line(hqwake.ClassWaiting+"·permission", "api:0.0 (%7)", `title:"run the tests?"`),
			hqwake.Line(hqwake.ClassDone, "web:2.0 (%11)", "3m",
				`goal:"fix the login bug"`, `tail:"tests pass"`) + " · #a3f1c2",
		}, "\n")
	},
}

// Render returns the registered rendering for id.
func Render(id string) (string, bool) {
	f, ok := Examples[id]
	if !ok {
		return "", false
	}
	return f(), true
}

// marker opens a rendered region: `<!-- gtmux:rendered <id> -->` on its own line,
// immediately followed by a fenced code block whose CONTENT is the region.
//
// There is no closing marker: the fence already delimits the block, and a second
// delimiter is one more thing that can disagree with the first. One HTML comment is also
// all a reader ever sees of the machinery (it renders to nothing).
const marker = "<!-- gtmux:rendered "

// fence opens/closes a markdown code block.
const fence = "```"

// Region is one marked example found in a document.
type Region struct {
	ID       string
	Body     string // the fence's content, verbatim
	from, to int    // body line bounds [from, to) in the source's lines
}

// Regions parses every marked region in a markdown source, in order. A marker that is
// not followed by a fence is an error rather than a skip: it means someone meant to mark
// an example and the checking silently would not have happened.
func Regions(src string) ([]Region, error) {
	lines := strings.Split(src, "\n")
	var out []Region
	for i := 0; i < len(lines); i++ {
		id, ok := markerID(lines[i])
		if !ok {
			continue
		}
		if i+1 >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[i+1]), fence) {
			return nil, fmt.Errorf("region %q: the marker must be followed by a ``` fence", id)
		}
		end := -1
		for j := i + 2; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == fence {
				end = j
				break
			}
		}
		if end < 0 {
			return nil, fmt.Errorf("region %q: unterminated fence", id)
		}
		out = append(out, Region{
			ID:   id,
			Body: strings.Join(lines[i+2:end], "\n"),
			from: i + 2, to: end,
		})
		i = end
	}
	return out, nil
}

// markerID reads a region id out of a marker line ("" + false when it isn't one).
func markerID(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, marker) || !strings.HasSuffix(t, "-->") {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, marker), "-->"))
	return id, id != ""
}

// Rewrite returns src with every marked region's body replaced by its registered
// rendering — the write half of the loop (`make docs-fix`), never run by CI. An
// unregistered id is left alone; Check is what reports it, so a rewrite can never
// quietly delete an example whose registry entry went missing.
func Rewrite(src string) (string, error) {
	regions, err := Regions(src)
	if err != nil {
		return "", err
	}
	lines := strings.Split(src, "\n")
	var out []string
	prev := 0
	for _, r := range regions {
		rendered, ok := Render(r.ID)
		if !ok {
			continue
		}
		out = append(out, lines[prev:r.from]...)
		out = append(out, strings.Split(rendered, "\n")...)
		prev = r.to
	}
	out = append(out, lines[prev:]...)
	return strings.Join(out, "\n"), nil
}

// Mismatch is one region whose documented body differs from what the code produces.
type Mismatch struct {
	ID         string
	Documented string
	Produced   string
	Reason     string // set when the region/registry pairing itself is broken
}

// Check compares every marked region in src against the registry, and reports every
// registered id that src does not mark. A dead registry entry is drift too: it means the
// example it guarded is gone, and nobody noticed the guard stopped guarding.
func Check(src string) ([]Mismatch, error) {
	regions, err := Regions(src)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var bad []Mismatch
	for _, r := range regions {
		seen[r.ID] = true
		produced, ok := Render(r.ID)
		if !ok {
			bad = append(bad, Mismatch{ID: r.ID, Documented: r.Body,
				Reason: "no registry entry — the doc marks an example nothing renders"})
			continue
		}
		if r.Body != produced {
			bad = append(bad, Mismatch{ID: r.ID, Documented: r.Body, Produced: produced,
				Reason: "the documented example is not what the code produces"})
		}
	}
	for id := range Examples {
		if !seen[id] {
			bad = append(bad, Mismatch{ID: id, Produced: mustRender(id),
				Reason: "registered but no region marks it — the example was removed or renamed"})
		}
	}
	return bad, nil
}

func mustRender(id string) string {
	s, _ := Render(id)
	return s
}
