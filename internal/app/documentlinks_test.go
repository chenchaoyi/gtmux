package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestConversationFileOpensWithoutPublicationAndKeepsIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.md")
	os.WriteFile(source, []byte("first"), 0600)
	record := func(message string) string {
		b, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "agent_message", "message": message}})
		return string(b)
	}
	desktopRollout(t, os.Getenv("CODEX_HOME"), "conversation", "01", "codex-tui", `{"type":"event_msg","payload":{"type":"user_message","message":"Review"}}`, record("[report]("+source+")"))
	desktopRollout(t, os.Getenv("CODEX_HOME"), "other", "01", "codex-tui", `{"type":"event_msg","payload":{"type":"user_message","message":"Unrelated"}}`)
	before, _ := artifact.List()
	if len(before) != 0 {
		t.Fatal(before)
	}
	for _, scope := range [][2]string{{"other", ""}, {"conversation", "foreign"}} {
		if _, err := resolveConversationDocument(source, scope[0], scope[1]); err == nil {
			t.Fatal("scope crossed", scope)
		}
	}
	first, err := resolveConversationDocument(source, "conversation", "")
	if err != nil || first.Source != source {
		t.Fatal(first, err)
	}
	os.WriteFile(source, []byte("updated"), 0600)
	latest, err := resolveConversationDocument(source, "conversation", "")
	if err != nil || latest.ID == first.ID {
		t.Fatal("path did not refresh", latest, err)
	}
	fixed, err := resolveConversationDocument(first.Reference(), "conversation", "")
	if err != nil || fixed.ID != first.ID {
		t.Fatal("fixed link changed", err)
	}
	homeFile, err := resolveConversationDocument(source, "conversation", transcript.CodexHomeID())
	if err != nil || homeFile.HomeID != transcript.CodexHomeID() {
		t.Fatal(homeFile, err)
	}
	os.Remove(source)
	retained, err := resolveConversationDocument(source, "conversation", "")
	if err != nil || retained.ID != latest.ID {
		t.Fatal("missing source lost its existing snapshot", retained, err)
	}
	os.WriteFile(source, []byte("updated"), 0600)
	if err := artifact.Remove(latest.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveConversationDocument(source, "conversation", ""); err == nil {
		t.Fatal("removed copy recreated")
	}
}

func TestConversationFileRejectsUnlinkedPathsAndToolOrCodeReferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "private.md")
	os.WriteFile(source, []byte("not a delivered file"), 0600)
	message := "The path is " + source + ". `Click [hidden](" + source + ")`\n\n```markdown\n[hidden](" + source + ")\n```"
	payload, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "agent_message", "message": message}})
	desktopRollout(t, os.Getenv("CODEX_HOME"), "session", "01", "codex-tui", `{"type":"event_msg","payload":{"type":"user_message","message":"Question"}}`, string(payload))
	if _, err := resolveConversationDocument(source, "session", ""); err == nil {
		t.Fatal("non-clickable source was read")
	}
	rows, _ := artifact.List()
	if len(rows) != 0 {
		t.Fatal("refusal wrote a snapshot", rows)
	}
}

func TestConversationFileOpensClaudeRelativeLink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	session := "a29c685a-7a56-4c61-a735-ddf2b8fcd5f6"
	cwd := t.TempDir()
	source := filepath.Join(cwd, "report.md")
	if err := os.WriteFile(source, []byte("# Claude report"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".claude", "projects", "-fixture")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	log := `{"type":"user","message":{"role":"user","content":"Review"}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"[Report](report.md)"}]}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resume.Save("fixture:0.0", resume.Record{Agent: "claude", SessionID: session, Cwd: cwd}); err != nil {
		t.Fatal(err)
	}
	file, err := resolveConversationDocument("report.md", session, "")
	if err != nil || file.Source != source {
		t.Fatal(file, err)
	}
	if _, err := resolveConversationDocument("report.md", session, "foreign"); err == nil {
		t.Fatal("Claude link crossed home identity")
	}
}

func TestConversationImageEnvelopeUsesTheSameDocumentBoundary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "attached image.png")
	if err := os.WriteFile(source, []byte("synthetic image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	record := func(message string) string {
		b, _ := json.Marshal(map[string]any{"type": "event_msg", "payload": map[string]string{"type": "user_message", "message": message}})
		return string(b)
	}
	raw := `<image name=[Image #1] path="` + source + `"></image>Inspect this`
	desktopRollout(t, os.Getenv("CODEX_HOME"), "image-owner", "01", "codex-tui", record(raw))
	desktopRollout(t, os.Getenv("CODEX_HOME"), "image-literal", "01", "codex-tui", record("```xml\n"+raw+"\n```"))
	if _, err := resolveConversationDocument(source, "image-literal", ""); err == nil {
		t.Fatal("code example granted file access")
	}
	if _, err := resolveConversationDocument(source, "image-owner", "foreign"); err == nil {
		t.Fatal("image crossed home identity")
	}
	file, err := resolveConversationDocument(source, "image-owner", "")
	if err != nil || file.Source != source || file.Kind != "image" {
		t.Fatal(file, err)
	}
}
