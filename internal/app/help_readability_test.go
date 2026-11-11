package app

import (
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"strings"
	"testing"
)

func TestCAS006HelpKeepsLongSyntaxSeparateAndScreenshotsVisible(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang("en") })
	for _, width := range []string{"40", "60", "80"} {
		t.Setenv("COLUMNS", width)
		for _, lang := range []string{"zh", "en"} {
			i18n.SetLang(lang)
			text := usageText()
			limit := helpWidth()
			for _, line := range strings.Split(text, "\n") {
				if i18n.DispWidth(line) > limit {
					t.Fatalf("%s/%s help overflow: %q", width, lang, line)
				}
			}
			if !strings.Contains(text, "screenshot <operation> [--json]") {
				t.Fatal("shipped screenshot entry still hidden", text)
			}
			for _, name := range []string{"agents", "logs", "doctor"} {
				c := findCommand(name)
				joined := strings.Join(strings.Fields(text), " ")
				if !strings.Contains(joined, c.Name+" "+c.Args) {
					t.Fatalf("long command syntax lost: %s", name)
				}
				summary := c.EN
				if lang == "zh" {
					summary = c.ZH
				}
				if strings.Contains(text, c.Args+summary) {
					t.Fatal("description collides with syntax", name)
				}
			}
			detail := screenshotHelpText()
			for _, op := range []string{"capabilities", "sources --app", "capture --source-ref", "import --input", "annotate --capture", "edit --document", "target --pane", "send --image", "status ID", "--request-id", "--timeout"} {
				if !strings.Contains(detail, op) {
					t.Fatal("screenshot operation/constraint lost", lang, op)
				}
			}
			for _, line := range strings.Split(detail, "\n") {
				if i18n.DispWidth(line) > limit {
					t.Fatal("screenshot help overflow", lang, width, line)
				}
			}
			if lang == "zh" && (!strings.Contains(detail, "截图、标注") || !strings.Contains(detail, "无需 MCP")) {
				t.Fatal("screenshot help not localized")
			}
		}
	}
}
