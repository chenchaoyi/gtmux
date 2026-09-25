package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeKB(t *testing.T, topic, body string) {
	t.Helper()
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(), topic+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A dispatch whose goal matches live entries surfaces them, in either language. The echo
// ranks through the package's one retrieval, so a goal with no spaces in it works: it used
// to split the goal on spaces and substring-match, which returned nothing for Chinese.
func TestMatchKnowledgeSurfacesHits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeOps(t,
		kadd("pitfalls", "never inline backticks in a PR body, they execute", "use --body-file instead"),
		kadd("pitfalls", "an unrelated footgun about disk hygiene", "nothing to do with the above"),
		kadd("workflows", "release flow: tag vX.Y.Z then wait for the app job", "goreleaser ships the tarballs"),
		kadd("best-practices", "手机配对超时要有上限，不能一直转圈", "每一步都给 deadline"),
		kadd("corrections", "release was claimed done without running it", "a correction, never dispatch context"),
	)
	out := MatchKnowledge("/tmp/other", "cut a release build")
	if !strings.Contains(out, "release flow") {
		t.Errorf("an English goal should surface the workflow; got %q", out)
	}
	if !strings.Contains(out, "[workflows]") {
		t.Errorf("hits should be topic-tagged; got %q", out)
	}
	if !strings.Contains(out, "workflows/") {
		t.Errorf("hits should name the id so HQ can read the entry; got %q", out)
	}
	// The case the old substring match could never answer.
	zh := MatchKnowledge("/tmp/other", "修复手机配对超时")
	if !strings.Contains(zh, "手机配对超时") {
		t.Errorf("a Chinese goal returned nothing: %q", zh)
	}
	// best-practices joined the echo; corrections stays out of dispatch context.
	if strings.Contains(out, "corrections/") {
		t.Errorf("a correction reached a dispatch: %q", out)
	}
}

// The topics the echo covers are a decision, not an accident, so they are pinned.
func TestEchoTopicsAreADecision(t *testing.T) {
	in := map[string]bool{}
	for _, t := range knowledgeEchoTopics {
		in[t] = true
	}
	for _, want := range []string{"pitfalls", "workflows", "best-practices"} {
		if !in[want] {
			t.Errorf("%s should be echoed at dispatch", want)
		}
	}
	for _, out := range []string{"accounts", "corrections", "environment"} {
		if in[out] {
			t.Errorf("%s is not dispatch-time context", out)
		}
	}
}

// No match → empty string (a silent no-op), and a missing KB is not an error.
func TestMatchKnowledgeNoMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No KB files at all.
	if out := MatchKnowledge("/tmp/whatever", "do a thing"); out != "" {
		t.Errorf("missing KB must yield no echo; got %q", out)
	}
	writeKB(t, "pitfalls", "# Pitfalls\n\n- something about kubernetes and networking.\n")
	if out := MatchKnowledge("/tmp/whatever", "paint the fence"); out != "" {
		t.Errorf("a non-matching dispatch must yield no echo; got %q", out)
	}
}

// The echo is capped so a dispatch stays terse.
func TestMatchKnowledgeCapsLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var b strings.Builder
	b.WriteString("# Pitfalls\n\n")
	for i := 0; i < 20; i++ {
		b.WriteString("- release note number that mentions release again\n")
	}
	writeKB(t, "pitfalls", b.String())
	out := MatchKnowledge("/tmp/x", "release")
	if n := strings.Count(out, "\n    - "); n > knowledgeEchoMaxLines {
		t.Errorf("echo has %d hit lines, want ≤ %d", n, knowledgeEchoMaxLines)
	}
}
