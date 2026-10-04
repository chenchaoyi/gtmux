package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

var newSessionMu sync.Mutex
var errSessionNameExists = errors.New("session name already exists")
var errSessionRequestChanged = errors.New("request id already used with a different name")

type createdSession struct {
	Session string `json:"session"`
	PaneID  string `json:"pane_id"`
	Window  string `json:"window"`
	Pane    string `json:"pane"`
	Loc     string `json:"loc"`
}

func normalizedSessionName(name string) string {
	return strings.NewReplacer(".", "-", ":", "-").Replace(strings.TrimSpace(name))
}

func validSessionName(name string) bool {
	if len(name) > 256 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

const sessionReceiptFormat = "#{session_name}\t#{pane_id}\t#{window_index}\t#{pane_index}"

func sessionReceipt(text string) (createdSession, error) {
	parts := strings.Split(text, "\t")
	if len(parts) != 4 || parts[0] == "" || !strings.HasPrefix(parts[1], "%") {
		return createdSession{}, errors.New("could not read created session identity")
	}
	return createdSession{parts[0], parts[1], parts[2], parts[3], parts[0] + ":" + parts[2] + "." + parts[3]}, nil
}

// createDetachedSession is shared by local New and remote creation. No command is
// injected into an existing pane; tmux starts its configured default shell.
// Request tags are installed by new-session itself, so response loss or serve
// restart does not discard the receipt while the tmux session remains alive.
func createDetachedSession(name, requestID, cwd string) (createdSession, error) {
	newSessionMu.Lock()
	defer newSessionMu.Unlock()
	if !validSessionName(name) {
		return createdSession{}, errors.New("invalid session name")
	}
	name = normalizedSessionName(name)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	if requestID != "" {
		text, err := tmux.Run("list-panes", "-a", "-F", "#{GTMUX_CREATE_ID}\t#{GTMUX_CREATE_NAME_SHA}\t"+sessionReceiptFormat)
		if err != nil && !noSessionServer(err) {
			return createdSession{}, fmt.Errorf("could not check session receipt: %w", err)
		}
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) == 3 && parts[0] == requestID {
				if parts[1] != hash {
					return createdSession{}, errSessionRequestChanged
				}
				return sessionReceipt(parts[2])
			}
		}
	}
	if name != "" && tmux.OK("has-session", "-t", "="+name) {
		return createdSession{}, errSessionNameExists
	}
	args := []string{"new-session", "-d", "-P", "-F", sessionReceiptFormat}
	if name != "" {
		args = append(args, "-s", name)
	}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	if requestID != "" {
		args = append(args, "-e", "GTMUX_CREATE_ID="+requestID, "-e", "GTMUX_CREATE_NAME_SHA="+hash)
	}
	text, err := tmux.Run(args...)
	if err != nil {
		return createdSession{}, err
	}
	return sessionReceipt(text)
}

// sessionStartDir is where a session gtmux creates without being told a directory
// starts. Left to itself tmux starts it in the creating process's directory, which for
// the menu-bar app and `gtmux serve` (and anything launchd starts) is "/": a shell, and
// any agent then run in it, at the root of the disk. Home is where a terminal starts.
// A real working directory — `gtmux new` typed inside a project — is kept: "" lets tmux
// use it.
func sessionStartDir() string {
	if wd, err := os.Getwd(); err == nil && wd != "/" {
		return ""
	}
	return state.Home()
}

// An absent server is the normal first-create case. Other lookup errors must not
// be treated as a missing receipt, which could create another session on retry.
func noSessionServer(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	message := string(exit.Stderr)
	return strings.Contains(message, "no server running on ") ||
		strings.Contains(message, "no sessions") ||
		(strings.Contains(message, "error connecting to ") && strings.Contains(message, "No such file or directory"))
}
