package terminalkeys

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/ansi"
)

var questionCount = regexp.MustCompile(`^\?\s+([1-9][0-9]{0,2})\s+questions?$`)

// CodexQuestions is a fallback for a collapsed live footer, not a question-log
// parser. The displayed hint supplies the resolved binding; unknown hints fail closed.
func CodexQuestions(agent, screen, binding string) []Action {
	if agent != "codex" || len(screen) > 128<<10 {
		return nil
	}
	lines := strings.Split(ansi.Strip(screen), "\n")
	if len(lines) > 18 {
		lines = lines[len(lines)-18:]
	}
	for i := 0; i+1 < len(lines); i++ {
		m := questionCount.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			continue
		}
		hint := strings.ToLower(strings.Join(strings.Fields(lines[i+1]), " "))
		var e Event
		switch strings.TrimSpace(hint) {
		case "⇧← to answer", "⇧ ← to answer", "shift+← to answer", "shift+left to answer":
			e = Event{"ArrowLeft", []string{"shift"}}
		case "alt+↑ to answer", "alt+up to answer", "⌥↑ to answer":
			e = Event{"ArrowUp", []string{"alt"}}
		default:
			continue
		}
		composer := false
		for _, line := range lines[i+2:] {
			if strings.HasPrefix(strings.TrimSpace(line), "› ") || strings.HasPrefix(strings.TrimSpace(line), "> ") {
				composer = true
				break
			}
		}
		if !composer {
			continue
		}
		count, _ := strconv.Atoi(m[1])
		sum := sha256.Sum256([]byte(binding + "\x00" + m[0] + "\x00" + hint))
		return []Action{{ID: "codex.view-question", Revision: hex.EncodeToString(sum[:]), Label: "View question", LabelZH: "查看问题", Count: count, Event: e}}
	}
	return nil
}
