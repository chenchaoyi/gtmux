package terminal

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmuxSurface is one terminal panel from `cmux tree --json`. The AppleScript
// `name` of a cmux terminal is often the literal "Terminal" (not the OSC/tab
// title), so focus/IsViewing/TabOrder key off this tree instead: tty maps a
// tmux client to its panel, and title is the real dynamic title when present.
type cmuxSurface struct {
	ID      string
	Title   string
	TTY     string // bare, e.g. "ttys019"
	Focused bool
}

var errNoCmuxCLI = errors.New("cmux CLI not found")

// cmuxCLI runs the bundled/on-PATH cmux CLI and returns stdout. Overridable in tests.
var cmuxCLI = func(args ...string) ([]byte, error) {
	bin := findCmuxCLI()
	if bin == "" {
		return nil, errNoCmuxCLI
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// clientTTYsFor returns the bare TTYs of tmux clients attached to session.
var clientTTYsFor = func(session string) []string {
	if session == "" || tmux.Bin == "" {
		return nil
	}
	var out []string
	for _, line := range tmux.Lines("list-clients", "-t", session, "-F", "#{client_tty}") {
		if t := bareTTY(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// allClientTTYSessions maps bare TTY → session for every attached client.
var allClientTTYSessions = func() map[string]string {
	m := map[string]string{}
	if tmux.Bin == "" {
		return m
	}
	for _, line := range tmux.Lines("list-clients", "-F", "#{client_tty}\t#{client_session}") {
		tty, sess, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		t := bareTTY(tty)
		sess = strings.TrimSpace(sess)
		if t == "" || sess == "" {
			continue
		}
		if _, exists := m[t]; !exists {
			m[t] = sess
		}
	}
	return m
}

func bareTTY(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "/dev/")
}

func findCmuxCLI() string {
	if v := strings.TrimSpace(os.Getenv("GTMUX_CMUX")); v != "" {
		return v
	}
	if p, err := exec.LookPath("cmux"); err == nil {
		return p
	}
	home := state.Home()
	for _, p := range []string{
		"/Applications/cmux.app/Contents/Resources/bin/cmux",
		filepath.Join(home, "Applications/cmux.app/Contents/Resources/bin/cmux"),
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// listCmuxSurfaces reads every terminal surface from `cmux tree --json`.
// activeID is the focused surface when cmux reports one.
func listCmuxSurfaces() (surfaces []cmuxSurface, activeID string, err error) {
	out, err := cmuxCLI("--json", "--id-format", "uuids", "tree", "--all")
	if err != nil {
		return nil, "", err
	}
	return parseCmuxTree(out)
}

func parseCmuxTree(raw []byte) ([]cmuxSurface, string, error) {
	var tree struct {
		Active *struct {
			SurfaceID string `json:"surface_id"`
		} `json:"active"`
		Windows []struct {
			Workspaces []struct {
				Panes []struct {
					Surfaces []struct {
						ID      string `json:"id"`
						Title   string `json:"title"`
						TTY     string `json:"tty"`
						Type    string `json:"type"`
						Focused bool   `json:"focused"`
						Active  bool   `json:"active"`
					} `json:"surfaces"`
				} `json:"panes"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, "", err
	}
	var surfaces []cmuxSurface
	for _, w := range tree.Windows {
		for _, ws := range w.Workspaces {
			for _, p := range ws.Panes {
				for _, s := range p.Surfaces {
					if s.Type != "" && s.Type != "terminal" {
						continue
					}
					if s.ID == "" {
						continue
					}
					surfaces = append(surfaces, cmuxSurface{
						ID:      s.ID,
						Title:   s.Title,
						TTY:     bareTTY(s.TTY),
						Focused: s.Focused || s.Active,
					})
				}
			}
		}
	}
	activeID := ""
	if tree.Active != nil {
		activeID = tree.Active.SurfaceID
	}
	return surfaces, activeID, nil
}

// cmuxSurfaceIDForSession picks the cmux terminal panel that hosts `session`:
// prefer an attached client's TTY, then a tree title that names the session.
func cmuxSurfaceIDForSession(session string) string {
	if session == "" {
		return ""
	}
	surfaces, _, err := listCmuxSurfaces()
	if err != nil || len(surfaces) == 0 {
		return ""
	}
	want := map[string]bool{}
	for _, t := range clientTTYsFor(session) {
		want[t] = true
	}
	for _, s := range surfaces {
		if s.TTY != "" && want[s.TTY] {
			return s.ID
		}
	}
	for _, s := range surfaces {
		if ghostty.TitleMatchesSession(s.Title, session) {
			return s.ID
		}
	}
	return ""
}
