package hq

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

//go:embed playbook/*/*.md
var playbookFiles embed.FS

var runbookNames = []string{"startup.md", "dispatch.md", "workspace.md", "maintenance.md"}

func playbookText(lang, name string) string {
	body, err := playbookFiles.ReadFile("playbook/" + lang + "/" + name)
	if err != nil {
		panic(err) // A missing embedded edition is a build/test error.
	}
	return string(body)
}

func runbookDir(lang string) string {
	return fmt.Sprintf("runbooks/v%d/%s", hqPlaybookVersion, lang)
}

// Publish the immutable release files before publishing an entry that links to
// them. Retain older editions for existing sessions and saved charters.
func ensureRunbooks(lang string) (changed bool, err error) {
	dir := state.HQHome()
	for _, part := range strings.Split(runbookDir(lang), "/") {
		dir = filepath.Join(dir, part)
		if err := state.CheckPrivateDirectory(dir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return false, fmt.Errorf("managed HQ runbook directory %s: %w", dir, err)
		}
	}
	for _, name := range runbookNames {
		path := filepath.Join(state.HQHome(), runbookDir(lang), name)
		body := []byte(playbookText(lang, name))
		f, err := state.OpenPrivateRead(path)
		if errors.Is(err, os.ErrNotExist) {
			if err := state.WritePrivateFile(path, body); err != nil {
				return changed, fmt.Errorf("managed HQ runbook %s: %w", path, err)
			}
			changed = true
			continue
		}
		if err != nil {
			return changed, fmt.Errorf("managed HQ runbook %s: %w", path, err)
		}
		got, err := io.ReadAll(io.LimitReader(f, int64(len(body))+1))
		_ = f.Close()
		if err != nil {
			return changed, fmt.Errorf("managed HQ runbook %s: %w", path, err)
		}
		if !bytes.Equal(got, body) {
			return changed, fmt.Errorf("managed HQ runbook differs from the shipped copy: %s", path)
		}
	}
	return changed, nil
}

// playbookIn renders the installed entry in the selected language. Claude
// follows the LOCAL import; clients without imports read it as directed by the
// entry. Runbooks are ordinary links, opened only on the indicated triggers.
func playbookIn(lang string) string {
	if lang != "zh" {
		lang = "en"
	}
	entry := strings.ReplaceAll(playbookText(lang, "entry.md"), "RUNBOOKS/", runbookDir(lang)+"/")
	return playbookMarker(hqPlaybookVersion, lang) + "\n\n" + entry + "\n@LOCAL.md\n"
}
