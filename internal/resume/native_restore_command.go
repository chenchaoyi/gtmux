package resume

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// NativeRestoreCommand is deliberately separate from CommandWithBridge.
// argv is the real executable recipe; command additionally pins HOME and cwd.
func NativeRestoreCommand(r Record, binary, home string, embedded bool, proxy string) (argv []string, command string, err error) {
	if r.Agent != "codex" || r.Launcher != "" || r.SessionID == "" ||
		strings.ContainsAny(r.SessionID, "\x00\r\n") || !filepath.IsAbs(binary) ||
		filepath.Base(binary) != "codex" || !filepath.IsAbs(home) ||
		r.HomeID != transcript.CodexHomeIDForPath(home) || r.HomeID != transcript.CodexHomeID() || strings.ContainsRune(proxy, 0) {
		return nil, "", errors.New("native restore scope unavailable")
	}
	argv = []string{binary, "--no-alt-screen"}
	if embedded {
		argv = append(argv, "--no-daemon")
	}
	argv = append(argv, "resume", r.SessionID)
	var quoted []string
	for _, word := range argv {
		quoted = append(quoted, shellSingleQuote(word))
	}
	environment := []string{"CODEX_HOME=" + shellSingleQuote(home)}
	if proxy != "" {
		environment = append(environment, "HTTPS_PROXY="+shellSingleQuote(proxy), "HTTP_PROXY="+shellSingleQuote(proxy))
	}
	// Environment assignments belong to the actual env/executable command, never cd.
	command = "env " + strings.Join(environment, " ") + " " + strings.Join(quoted, " ")
	if r.Cwd != "" {
		q := shellSingleQuote(r.Cwd)
		// Native recovery must not fall through to an unrelated cwd if the saved directory disappears.
		command = "cd -- " + q + " && " + command
	}
	return argv, command, nil
}
