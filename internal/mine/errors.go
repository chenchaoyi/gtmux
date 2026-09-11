package mine

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Error signatures. A Bash tool result yields at most ONE line — the first that reads as
// a hard failure — and that line is normalized so the same footgun hit with different
// numbers, paths, or hashes tallies as one. A test runner's "N failed" summary is
// deliberately NOT an error signature: a red test is the ordinary shape of work, not a
// footgun, and it would drown everything else.
var (
	// hardErrorRe is the strong pattern: lines that only ever appear when something
	// refused to work. Deliberately narrower than "error" anywhere.
	hardErrorRe = regexp.MustCompile(`(?i)(command not found|no such file or directory|permission denied|unable to (locate|find|connect)|cannot (find|open|connect|stat)|failed to |fatal:|panic:|segmentation fault|is not recognized|connection refused|timed out|error: )`)
	// errorDeny drops lines that match hardErrorRe but are not footguns: test-runner and
	// linter summaries, a bare exit code, and a line that is DATA rather than a message
	// (a JSON fragment quoting an error is the transcript of something, not a failure —
	// on the machine this was tuned on, one such fragment tallied in ten sessions).
	errorDeny = regexp.MustCompile(`(?i)(tests?:|passed|✓|--- (PASS|FAIL)|^\s*(FAIL|ERROR)\s*$|\d+ problems?|^exit (code|status) \d+$|^\s*["{\[]|^\s*[\w-]+"?\s*:\s*"|^\s*⏺ API Error)`)
	numRe     = regexp.MustCompile(`\b\d+\b`)
	hexRe     = regexp.MustCompile(`\b[0-9a-f]{7,}\b`)
	pathRe    = regexp.MustCompile(`(/[\w.\-~@]+)+`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

// errorSignature returns ("", false) when the tool output holds no hard failure.
func errorSignature(output string) (string, bool) {
	for _, ln := range strings.Split(output, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || !hardErrorRe.MatchString(ln) || errorDeny.MatchString(ln) {
			continue
		}
		return normalizeError(ln), true
	}
	return "", false
}

const sigRunes = 120

func normalizeError(s string) string {
	s = pathRe.ReplaceAllString(s, "<path>")
	s = hexRe.ReplaceAllString(s, "<hex>")
	s = numRe.ReplaceAllString(s, "N")
	s = spaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > sigRunes {
		s = string([]rune(s)[:sigRunes])
	}
	return s
}
