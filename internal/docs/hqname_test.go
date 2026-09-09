package docs

// HQ has ONE name in anything a user reads.
//
// It had four: 参谋长 and 中控 in Chinese, "HQ", "the supervisor" and "CHIEF OF STAFF" in
// English — often two of them on one screen (the phone's header said 参谋长 where the
// English said HQ; the menu bar's role banner said CHIEF OF STAFF; a sheet title said
// "Ask the supervisor"). The operator settled it on 2026-09-09: it is HQ, both languages,
// everywhere they can see it.
//
// This is a TEST rather than a grep in check-design.sh because the question — "is this
// inside a string literal?" — needs a scanner, not a regex. The shell version flagged
// three trailing comments as violations, which is the same class of false answer that
// makes a gate get ignored.
//
// What is deliberately NOT scanned:
//
//   - COMMENTS anywhere. Reasoning about the chief-of-staff role is how the code explains
//     itself, and the old words are the accurate words for that reasoning.
//   - `mobileapp/src/releaseNotes.ts` — the archive of what PAST versions said. Rewriting
//     it would falsify shipped history.
//   - the HQ charter (`internal/hq/hq.go`, `internal/hq/playbook_zh.go`) — instructions HQ
//     reads about its own role, not a label on a screen. The metaphor there is what
//     teaches the behaviour; renaming it would cost something real.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var hqOldNames = []string{"参谋长", "中控", "CHIEF OF STAFF", "the supervisor"}

// stringLiterals returns the contents of every quoted span on a line, with any trailing
// line comment removed first. Quote-aware, so a `//` inside a string does not truncate it
// and a quote inside a comment does not open one.
func stringLiterals(line string) []string {
	var out []string
	r := []rune(line)
	for i := 0; i < len(r); i++ {
		switch {
		case r[i] == '/' && i+1 < len(r) && r[i+1] == '/':
			return out // a real comment: nothing after it is a literal
		case r[i] == '"' || r[i] == '\'' || r[i] == '`':
			q := r[i]
			j := i + 1
			var b strings.Builder
			for ; j < len(r); j++ {
				if r[j] == '\\' && q != '`' {
					j++
					continue
				}
				if r[j] == q {
					break
				}
				b.WriteRune(r[j])
			}
			out = append(out, b.String())
			i = j
		}
	}
	return out
}

func TestHQIsCalledHQEverywhereAUserCanSeeIt(t *testing.T) {
	root := filepath.Join("..", "..")
	skip := []string{
		"releaseNotes.ts",
		filepath.Join("internal", "hq", "hq.go"),
		filepath.Join("internal", "hq", "playbook_zh.go"),
	}
	var bad []string
	for _, tree := range []string{"internal", filepath.Join("macapp", "Sources"), filepath.Join("mobileapp", "src")} {
		_ = filepath.Walk(filepath.Join(root, tree), func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			ext := filepath.Ext(p)
			if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".swift" {
				return nil
			}
			rel := strings.TrimPrefix(p, root+string(filepath.Separator))
			if strings.HasSuffix(p, "_test.go") || strings.Contains(p, ".test.") {
				return nil
			}
			for _, s := range skip {
				if strings.Contains(rel, s) {
					return nil
				}
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for n, line := range strings.Split(string(b), "\n") {
				for _, lit := range stringLiterals(line) {
					for _, old := range hqOldNames {
						if strings.Contains(lit, old) {
							bad = append(bad, rel+":"+itoa(n+1)+"  "+old+"  in  "+trim(lit, 60))
						}
					}
				}
			}
			return nil
		})
	}
	if len(bad) > 0 {
		t.Errorf("HQ is called something else in %d user-facing string(s) — it is HQ, both languages:\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
