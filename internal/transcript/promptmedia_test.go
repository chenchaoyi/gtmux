package transcript

import (
	"strings"
	"testing"
)

func TestPromptImageEnvelopeIsOneSharedMarkdownLink(t *testing.T) {
	raw := `<image name=[Image #1] path="/tmp/a screenshot.png"></image>Look here`
	got, ok := CleanUserPrompt(raw)
	if !ok || got != "![Image #1](/tmp/a%20screenshot.png)\nLook here" {
		t.Fatalf("%q %v", got, ok)
	}
	if again := normalizePromptMedia(got); again != got {
		t.Fatal("not idempotent")
	}
	for _, raw := range []string{"```xml\n" + raw + "\n```", "~~~xml\n" + raw + "\n~~~", "````xml\n```\n" + raw + "\n````", "```xml\n```not-a-closing-fence\n" + raw + "\n```", "    " + raw, "`" + raw + "`", strings.ReplaceAll(raw, "/tmp/a screenshot.png", "https://example.test/a.png"), strings.ReplaceAll(raw, "/tmp/a screenshot.png", "//host/a.png")} {
		if got := normalizePromptMedia(raw); got != raw {
			t.Fatalf("literal or external envelope changed: %q", got)
		}
	}
}

func TestSplitCodexImageEnvelopeKeepsReferenceAndHumanText(t *testing.T) {
	raw := "<image name=[Image #1] path=\"/tmp/a screenshot.png\">\n</image>\n[Image #1]"
	got, ok := CleanUserPrompt(raw)
	if !ok || got != "![Image #1](/tmp/a%20screenshot.png)\n\n[Image #1]" {
		t.Fatalf("%q %v", got, ok)
	}
	for _, literal := range []string{"```xml\n" + raw + "\n```", "~~~\n" + raw + "\n~~~", "    " + raw,
		strings.ReplaceAll(raw, "/tmp/a screenshot.png", "https://example.test/a.png"),
		strings.ReplaceAll(raw, "\n</image>", "\nnot an empty envelope\n</image>")} {
		if got := normalizePromptMedia(literal); got != literal {
			t.Fatalf("literal changed: %q", got)
		}
	}
}
