package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func kadd(topic, title, body string) knowledgeOp {
	return knowledgeOp{Op: knowledgeOpAdd, ID: topic + "/" + slug(title), Topic: topic,
		Title: title, Body: body, At: 1_755_000_000, Seq: 6650}
}

func TestRenderTopicShapeAndDeterminism(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	live := []knowledgeOp{
		kadd("pitfalls", "wrangler TLS-resets from the office", "retry the call; see gtmux repo notes"),
		kadd("pitfalls", "long body entry", strings.Repeat("x", knowledgeInlineBodyMax+1)),
		kadd("workflows", "release flow", ""), // another topic — must not leak in
	}
	live[0].Capture, live[0].Task, live[0].Pane = "pitfalls/wrangler-tls", "t-9", "%21"

	got := renderTopic("pitfalls", live)
	if !strings.HasPrefix(got, knowledgeRenderMarker+"\n") {
		t.Fatalf("render must lead with the gtmux-owned marker:\n%s", got)
	}
	// A short body flattens into the bullet (the echo greps bullets).
	if !strings.Contains(got, "- **wrangler TLS-resets from the office** — retry the call") {
		t.Errorf("short body must flatten into the bullet:\n%s", got)
	}
	// A long body indents beneath its bullet.
	if !strings.Contains(got, "- **long body entry**\n    x") {
		t.Errorf("long body must indent under the bullet:\n%s", got)
	}
	// The provenance footer names the evidence, indented (never a bullet).
	if !strings.Contains(got, "  · pitfalls/wrangler-tls-resets-from-the-office · 2025-08-12 · seq 6650 · capture pitfalls/wrangler-tls · task t-9 · pane %21") {
		t.Errorf("provenance footer missing or wrong:\n%s", got)
	}
	if strings.Contains(got, "release flow") {
		t.Errorf("another topic's entry leaked into the render:\n%s", got)
	}
	if again := renderTopic("pitfalls", live); again != got {
		t.Fatal("render must be deterministic")
	}
}

func TestMigrationPreservesHandWrittenBytes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	hand := "# Pitfalls\n\n- the gtmux release tag needs a user: block\n- 200KB of paid-for lessons\n"
	if err := os.WriteFile(topicPath("pitfalls"), []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}

	live := []knowledgeOp{kadd("pitfalls", "new lesson", "")}
	if err := writeTopicRender("pitfalls", live, 500); err != nil {
		t.Fatal(err)
	}
	// The hand-written bytes moved verbatim.
	b, err := os.ReadFile(filepath.Join(knowledgeLegacyDir(), "pitfalls.md"))
	if err != nil || string(b) != hand {
		t.Fatalf("legacy content must survive byte-for-byte; err=%v got:\n%s", err, b)
	}
	// The render links to it and carries the new entry.
	r, _ := os.ReadFile(topicPath("pitfalls"))
	if !strings.Contains(string(r), "legacy/pitfalls.md") || !strings.Contains(string(r), "new lesson") {
		t.Fatalf("render must link legacy and carry the entry:\n%s", r)
	}
	// Idempotent: a second render does not re-migrate or duplicate.
	if err := writeTopicRender("pitfalls", live, 600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(knowledgeLegacyDir())
	if len(entries) != 1 {
		t.Fatalf("a second mutation must not re-migrate; legacy holds %d files", len(entries))
	}
	// The echo still reaches the legacy lesson AND the rendered one.
	echo := MatchKnowledge("/Users/x/gtmux", "fix the release lesson")
	if !strings.Contains(echo, "user: block") {
		t.Errorf("legacy bullet lost from the dispatch echo:\n%s", echo)
	}
	echo2 := MatchKnowledge("/Users/x/whatever", "apply the new lesson now")
	if !strings.Contains(echo2, "new lesson") {
		t.Errorf("rendered bullet not consultable:\n%s", echo2)
	}
}

func TestMigrationDropsUntouchedPlaceholder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := hqKnowledgeSeeds["pitfalls.md"]
	if err := os.WriteFile(topicPath("pitfalls"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTopicRender("pitfalls", []knowledgeOp{kadd("pitfalls", "first", "")}, 500); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(knowledgeLegacyDir(), "pitfalls.md")); !os.IsNotExist(err) {
		t.Fatal("an untouched placeholder holds nothing worth a legacy file")
	}
}

func TestKnowledgeDriftDetection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	live := []knowledgeOp{kadd("pitfalls", "a lesson", "")}
	if err := writeTopicRender("pitfalls", live, 500); err != nil {
		t.Fatal(err)
	}
	if d := knowledgeDrift(live); len(d) != 0 {
		t.Fatalf("a clean render must pass, got drift %v", d)
	}
	// A hand edit to a rendered file is DETECTED, not absorbed.
	f, _ := os.OpenFile(topicPath("pitfalls"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("- a hand-added line\n")
	_ = f.Close()
	d := knowledgeDrift(live)
	if len(d) != 1 || !strings.HasSuffix(d[0], "pitfalls.md") {
		t.Fatalf("drift must name the edited file, got %v", d)
	}
	// A hand-written (never-rendered) topic is not drift — it is pre-migration.
	if err := os.WriteFile(topicPath("workflows"), []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := knowledgeDrift(live); len(d) != 1 {
		t.Fatalf("a pre-ledger file is not drift, got %v", d)
	}
}

// The dispatch echo's line cap holds across BOTH sources: renders first, legacy after,
// and the total never exceeds knowledgeEchoMaxLines.
func TestKnowledgeEchoCapSpansRenderAndLegacy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(knowledgeLegacyDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "- legacy gtmux lesson alpha\n- legacy gtmux lesson beta\n" +
		"- legacy gtmux lesson gamma\n- legacy gtmux lesson delta\n"
	if err := os.WriteFile(filepath.Join(knowledgeLegacyDir(), "pitfalls.md"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	live := []knowledgeOp{
		kadd("pitfalls", "gtmux rendered lesson one", ""),
		kadd("pitfalls", "gtmux rendered lesson two", ""),
	}
	if err := writeTopicRender("pitfalls", live, 500); err != nil {
		t.Fatal(err)
	}
	echo := MatchKnowledge("/Users/x/gtmux", "")
	lines := 0
	for _, l := range strings.Split(echo, "\n") {
		if strings.Contains(l, "[pitfalls]") {
			lines++
		}
	}
	if lines != knowledgeEchoMaxLines {
		t.Fatalf("echo lines = %d, want capped at %d:\n%s", lines, knowledgeEchoMaxLines, echo)
	}
	if !strings.Contains(echo, "rendered lesson one") {
		t.Errorf("renders must be consulted before legacy:\n%s", echo)
	}
}
