// Dispatch-time knowledge echo (hq-capture-loop PR3, the consult half-loop's tool layer).
// The capture layers (① mandatory verdict, ② `gtmux capture`) FILL the knowledge base;
// this SPENDS it. At `gtmux spawn` / dispatch, gtmux matches the pitfalls / workflows
// topics against the target repo (the cwd's base name) and the goal's keywords, and
// surfaces the hits at dispatch time — so captured knowledge reaches the moment work
// starts as a tool guarantee, not something HQ must remember to relay every time. No
// match → empty string (a silent no-op); this reads only its own knowledge files.
package hq

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// knowledgeEchoTopics are the two topics worth surfacing to a worker at launch — the
// footguns to avoid and the procedures to follow. Accounts/corrections are not
// dispatch-time context.
var knowledgeEchoTopics = []string{"pitfalls", "workflows"}

// knowledgeEchoMaxLines caps the echo so a dispatch stays terse.
const knowledgeEchoMaxLines = 4

// MatchKnowledge returns a short, human-readable summary of the pitfalls/workflows
// entries that match the target repo (filepath.Base(cwd)) or a goal keyword, or "" when
// nothing matches. It is advisory and read-only.
func MatchKnowledge(cwd, goal string) string {
	repo := strings.ToLower(filepath.Base(strings.TrimSpace(cwd)))
	if repo == "." || repo == "/" || repo == "" {
		repo = ""
	}
	keywords := goalKeywords(goal)
	if repo == "" && len(keywords) == 0 {
		return ""
	}

	var hits []string
	seen := map[string]bool{}
	for _, topic := range knowledgeEchoTopics {
		for _, line := range matchingBullets(filepath.Join(hqKnowledgeDir(), topic+".md"), repo, keywords) {
			key := topic + "|" + line
			if seen[key] {
				continue
			}
			seen[key] = true
			hits = append(hits, "    - ["+topic+"] "+line)
			if len(hits) >= knowledgeEchoMaxLines {
				break
			}
		}
		if len(hits) >= knowledgeEchoMaxLines {
			break
		}
	}
	if len(hits) == 0 {
		return ""
	}
	head := "• KB hits for this dispatch"
	if repo != "" {
		head += " (" + repo + ")"
	}
	return head + ":\n" + strings.Join(hits, "\n")
}

// matchingBullets returns the bullet lines ("- " / "* ") of a topic file that contain the
// repo token or any goal keyword (case-insensitive). Non-bullet prose is skipped so the
// echo is a list of concrete lessons, not headings.
func matchingBullets(path, repo string, keywords []string) []string {
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
		text := strings.TrimSpace(raw[2:])
		if text == "" {
			continue
		}
		low := strings.ToLower(text)
		if (repo != "" && strings.Contains(low, repo)) || containsAnyKeyword(low, keywords) {
			out = append(out, snipLine(text))
		}
	}
	return out
}

func containsAnyKeyword(low string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// goalKeywords extracts the distinctive words of a goal: lowercased, length ≥ 4, deduped,
// with a few common filler words dropped so a match means something.
func goalKeywords(goal string) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "into": true, "that": true,
		"this": true, "from": true, "then": true, "make": true, "please": true, "also": true,
		"when": true, "your": true, "each": true, "over": true, "onto": true,
	}
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Fields(strings.ToLower(goal)) {
		w = strings.Trim(w, ".,:;!?\"'()[]`")
		if len(w) < 4 || stop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// snipLine trims a matched bullet to a readable length for the echo.
func snipLine(s string) string {
	const max = 100
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "…"
}
