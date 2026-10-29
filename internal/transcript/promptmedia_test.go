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
