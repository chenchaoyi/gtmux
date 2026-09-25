// Dispatch-time knowledge echo (hq-capture-loop PR3, the consult half-loop's tool layer).
// The capture layers (① mandatory verdict, ② `gtmux capture`) FILL the knowledge base;
// this SPENDS it. At `gtmux spawn`, gtmux ranks the live entries against the target repo
// and the goal and prints the top few, so captured knowledge reaches the moment work
// starts as a tool guarantee rather than something HQ must remember to relay. It prints
// where HQ can see it; what reaches the worker stays HQ's call. No match → empty string.
//
// It used to grep the rendered topic files for a goal word, where the goal was split on
// spaces. A Chinese goal has no spaces, so the whole sentence became one keyword and the
// substring never matched: recall for a Chinese goal was zero. It ranks through the
// package's one retrieval now (retrieve.go), which tokenizes CJK as bigrams.
package knowledge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// knowledgeEchoTopics are the built-in topics worth surfacing to a worker at launch: the
// footguns to avoid, the procedures to follow, and the approaches that worked here.
// Accounts / corrections / environment are deliberately not dispatch-time context. Every
// DECLARED custom topic joins the echo (hq-open-topics): a user's own domain topics are
// exactly the lessons they want surfaced when work starts there.
var knowledgeEchoTopics = []string{"pitfalls", "workflows", "best-practices"}

// echoTopics is the effective echo set: the built-ins plus the ledger's custom
// declarations (best-effort — an unreadable ledger just means built-ins only).
func echoTopics() []string {
	out := append([]string{}, knowledgeEchoTopics...)
	if ops, err := readKnowledgeOps(); err == nil {
		for _, t := range customTopics(ops) {
			out = append(out, t.ID)
		}
	}
	return out
}

// knowledgeEchoMaxLines caps the echo so a dispatch stays terse.
const knowledgeEchoMaxLines = 4

// MatchKnowledge returns a short, human-readable summary of the entries that match the
// target repo and the goal, or "" when nothing does. Advisory and read-only.
func MatchKnowledge(cwd, goal string) string {
	repo := strings.ToLower(filepath.Base(strings.TrimSpace(cwd)))
	if repo == "." || repo == "/" || repo == "" {
		repo = ""
	}
	// The repo name rides in the query as one more token, so how much it counts is
	// decided by how rare it is: in its own repository it says nothing and weighs
	// nothing, and in a goal that names someone else's it is the strongest word there.
	text := strings.TrimSpace(repo + " " + goal)
	if text == "" {
		return ""
	}
	live, _ := liveKnowledge() // no ledger is not an error here; legacy may still answer
	topics := echoTopics()
	live = append(live, legacyEntries(topics)...)
	hits := search(live, Query{Text: text, Topics: topics, N: knowledgeEchoMaxLines})
	if len(hits) == 0 {
		return ""
	}
	lines := make([]string, 0, len(hits))
	for _, h := range hits {
		lines = append(lines, "    - ["+h.Topic+"] "+snipLine(h.Title)+" · "+h.ID)
	}
	head := "• KB hits for this dispatch"
	if repo != "" {
		head += " (" + repo + ")"
	}
	return head + ":\n" + strings.Join(lines, "\n")
}

// legacyEntries reads the pre-ledger topic files as entries, so the echo reaches a lesson
// that has not been migrated yet (hq-knowledge-ledger: during the incremental migration a
// lesson lives in exactly one of the two sides, and the echo must find it in either).
func legacyEntries(topics []string) []knowledgeOp {
	var out []knowledgeOp
	for _, t := range topics {
		for i, line := range bullets(filepath.Join(knowledgeLegacyDir(), t+".md")) {
			out = append(out, knowledgeOp{
				ID: fmt.Sprintf("legacy/%s#%d", t, i+1), Topic: t, Title: line, Legacy: true,
			})
		}
	}
	return out
}

// bullets returns a markdown file's bullet lines. Prose and headings are skipped: the echo
// is a list of concrete lessons, not an outline.
func bullets(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(raw, "- ") && !strings.HasPrefix(raw, "* ") {
			continue
		}
		if t := strings.TrimSpace(raw[2:]); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// snipLine trims a title to a readable length for the echo.
func snipLine(s string) string {
	const max = 100
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}
