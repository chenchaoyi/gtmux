package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeKB(t *testing.T, topic, body string) {
	t.Helper()
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hqKnowledgeDir(), topic+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A dispatch into a repo whose pitfalls/workflows have matching entries surfaces them.
func TestMatchKnowledgeSurfacesHits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeKB(t, "pitfalls", "# Pitfalls\n\n- gtmux serve: never inline backticks in a PR body — they execute.\n- unrelated footgun about something else entirely.\n")
	writeKB(t, "workflows", "# Workflows\n\n- release: tag vX.Y.Z then wait for the app job.\n")

	// Match by repo name (cwd base).
	out := MatchKnowledge("/Users/ccy/src/gtmux", "add a widget")
	if !strings.Contains(out, "gtmux serve") {
		t.Errorf("repo-name match should surface the gtmux pitfall; got %q", out)
	}
	if !strings.Contains(out, "[pitfalls]") {
		t.Errorf("hits should be topic-tagged; got %q", out)
	}

	// Match by goal keyword.
	out = MatchKnowledge("/tmp/other", "cut a release build")
	if !strings.Contains(out, "release:") {
		t.Errorf("keyword 'release' should surface the workflow; got %q", out)
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
