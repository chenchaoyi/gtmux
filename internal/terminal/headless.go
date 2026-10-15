package terminal

import (
	"errors"
	"strings"
)

// Headless never opens desktop applications. Sessions remain managed by tmux.
type Headless struct{}

var ErrNoDesktop = errors.New("desktop integration unavailable; connect with tmux attach or gtmux attach")

func (Headless) Name() string                      { return "tmux" }
func (Headless) FocusTab(string) (string, error)   { return "", ErrNoDesktop }
func (Headless) IsViewing(string) bool             { return false }
func (Headless) OpenWindow(string) (string, error) { return "", ErrNoDesktop }
func (Headless) TabOrder() []string                { return nil }
func (Headless) SpawnTabs(sessions []string, dry bool) (string, error) {
	lines := make([]string, 0, len(sessions))
	for _, s := range sessions {
		lines = append(lines, "tmux attach -t '"+strings.ReplaceAll(s, "'", "'\"'\"'")+"'")
	}
	if dry {
		return strings.Join(lines, "\n"), nil
	}
	return strings.Join(lines, "\n"), ErrNoDesktop
}
