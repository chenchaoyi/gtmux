package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/artifact"
)

func TestArtifactCommandRequiresIdentityAndPreservesOriginal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.html")
	os.WriteFile(source, []byte("<html>report</html>"), 0600)
	if code := cmdArtifact([]string{"add", source}); code != 2 {
		t.Fatal("missing identity accepted", code)
	}
	var code int
	out := captureStdout(t, func() { code = cmdArtifact([]string{"add", source, "--session-id", "session", "--json"}) })
	var file artifact.File
	if code != 0 || json.Unmarshal([]byte(out), &file) != nil || file.ID == "" || !strings.Contains(out, "gtmux-file:") {
		t.Fatal(code, out)
	}
	list := captureStdout(t, func() { code = cmdArtifact([]string{"list"}) })
	if code != 0 || !strings.Contains(list, file.ID) {
		t.Fatal(code, list)
	}
	if code = cmdArtifact([]string{"remove", file.ID}); code != 0 {
		t.Fatal(code)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "<html>report</html>" {
		t.Fatal("original changed", err)
	}
}
