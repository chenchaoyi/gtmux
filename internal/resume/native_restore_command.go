package resume

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// CodexRestoreBinaryEnv pins the verified executable for a managed restore.
const CodexRestoreBinaryEnv = "GTMUX_CODEX_RESTORE_BINARY"

// NativeRestoreCommand is deliberately separate from CommandWithBridge.
// argv is the real executable recipe; command additionally pins HOME and cwd.
func NativeRestoreCommand(r Record, binary, home string, embedded bool, proxy string) (argv []string, command string, err error) {
	if filepath.Base(binary) != "codex" || !validCodexRestoreScope(r, binary, home, proxy) {
		return nil, "", errors.New("native restore scope unavailable")
	}
	argv = []string{binary, "--no-alt-screen"}
	if embedded {
		argv = append(argv, "--no-daemon")
	}
	argv = append(argv, "resume", r.SessionID)
	return argv, codexRestoreShell(r.Cwd, home, proxy, argv, ""), nil
}

// ManagedCodexRestoreCommand resumes the same thread through a required bridge.
func ManagedCodexRestoreCommand(r Record, gtmux, binary, home, proxy string) (string, error) {
	if r.Frontend == "" || !filepath.IsAbs(gtmux) || strings.ContainsAny(gtmux, "\x00\r\n") ||
		!validCodexRestoreScope(r, binary, home, proxy) {
		return "", errors.New("managed restore scope unavailable")
	}
	argv := []string{gtmux, "codex", "--no-alt-screen", "resume", r.SessionID}
	return codexRestoreShell(r.Cwd, home, proxy, argv, binary), nil
}

func validCodexRestoreScope(r Record, binary, home, proxy string) bool {
	return r.Agent == "codex" && r.Launcher == "" && r.SessionID != "" &&
		!strings.ContainsAny(r.SessionID, "\x00\r\n") && filepath.IsAbs(binary) &&
		filepath.IsAbs(home) &&
		r.HomeID == transcript.CodexHomeIDForPath(home) && r.HomeID == transcript.CodexHomeID() && !strings.ContainsRune(proxy, 0)
}

func codexRestoreShell(cwd, home, proxy string, argv []string, managedBinary string) string {
	var quoted []string
	for _, word := range argv {
		quoted = append(quoted, shellSingleQuote(word))
	}
	environment := []string{"CODEX_HOME=" + shellSingleQuote(home)}
	if managedBinary != "" {
		environment = append(environment, CodexRestoreBinaryEnv+"="+shellSingleQuote(managedBinary))
	}
	if proxy != "" {
		environment = append(environment, "HTTPS_PROXY="+shellSingleQuote(proxy), "HTTP_PROXY="+shellSingleQuote(proxy))
	}
	// Environment assignments belong to the actual env/executable command, never cd.
	command := "env " + strings.Join(environment, " ") + " " + strings.Join(quoted, " ")
	if cwd != "" {
		q := shellSingleQuote(cwd)
		// Recovery must not fall through to an unrelated cwd if the saved directory disappears.
		command = "cd -- " + q + " && " + command
	}
	return command
}
