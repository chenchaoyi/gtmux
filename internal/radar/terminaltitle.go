package radar

import (
	"strings"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
)

// terminalTask only labels an already verified attachment. It grants no identity,
// status or input capability and never derives a title from conversation prose.
func terminalTask(a agentclient.Attachment, paneTitle, fallback string) string {
	if a.Agent != "dsh" || a.HomeID != dsh.HomeID() {
		return fallback
	}
	if s, err := terminalHistory(a.SessionID); err == nil && s.ID == a.SessionID && s.HomeID == a.HomeID && s.Origin != "subagent" && hqpane.SameDir(s.Cwd, a.Cwd) {
		if title := strings.Join(strings.Fields(s.Title), " "); title != "" {
			return title
		}
	}
	// A community TUI can publish a real title before DSH persists a title event.
	// Strip standalone decorative tokens, including emoji variation selectors.
	words := strings.Fields(paneTitle)
	for len(words) > 0 && titleDecoration(words[0]) {
		words = words[1:]
	}
	title := strings.Join(words, " ")
	switch strings.ToLower(title) {
	case "", "deepseek", "deepseek harness", "dsh", "dst":
		return fallback
	}
	return title
}

func titleDecoration(token string) bool {
	symbol := false
	for _, r := range token {
		if unicode.IsSymbol(r) {
			symbol = true
			continue
		}
		if !unicode.IsMark(r) && r != '\u200d' {
			return false
		}
	}
	return symbol
}
