package agentclient

import (
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/toolpath"
	"os"
	"path/filepath"
	"strings"
)

// Command selects a declared interactive client, rather than launching a Web
// server under a harness's generic CLI name. Other clients add an adapter here.
func Command(agent, client, sessionID string) (string, bool) {
	if !declared(agent, client) {
		return "", false
	}
	var argv []string
	switch client {
	case "dst":
		argv = []string{"env", "DSH_HOME=" + dsh.Home(), "DSH_TUI_NO_LAUNCHPAD=1", "dst", "--backend", "dsh"}
		if executable := toolpath.Look("dst"); executable != "" {
			argv[3] = executable
			// A Node shebang must find the runtime paired with this installation,
			// even when launched by a GUI with launchd's minimal PATH.
			if node := filepath.Join(filepath.Dir(executable), "node"); toolpath.IsExecutable(node) {
				path := os.Getenv("PATH")
				if path == "" {
					path = "/usr/bin:/bin:/usr/sbin:/sbin"
				}
				argv = append(argv[:1], append([]string{"PATH=" + filepath.Dir(executable) + string(os.PathListSeparator) + path}, argv[1:]...)...)
			}
		}
		if sessionID == "" {
			argv = append(argv, ".")
		}
	default:
		return "", false
	}
	if sessionID != "" {
		argv = append(argv, "--resume", sessionID)
	}
	for i := range argv {
		argv[i] = "'" + strings.ReplaceAll(argv[i], "'", `'\''`) + "'"
	}
	return strings.Join(argv, " "), true
}
