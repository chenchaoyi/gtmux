package server

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/ansi"
	"github.com/chenchaoyi/gtmux/internal/prompt"
)

// Interaction is a bounded observation, not a transcript or a status detector.
type Interaction struct {
	Text          string          `json:"text"`
	Options       []prompt.Option `json:"options"`
	AwaitingReply bool            `json:"awaiting_reply"`
	Truncated     bool            `json:"truncated"`
}

func interactionText(text string) (string, bool) {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(text))
	lines := strings.Split(strings.TrimRight(text, "\n \t"), "\n")
	cut := len(lines) > 80
	if cut {
		lines = lines[len(lines)-80:]
	}
	runes := []rune(strings.Join(lines, "\n"))
	if len(runes) > 16000 {
		runes = runes[len(runes)-16000:]
		cut = true
	}
	return string(runes), cut
}

func (s *Server) handleInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	if !s.mayReachPane(w, r, id) {
		return
	}
	if s.deps.PaneScreen == nil {
		writeJSON(w, http.StatusNotImplemented, errBody("current screen unavailable"))
		return
	}
	text, ok := s.deps.PaneScreen(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("pane not found"))
		return
	}
	view := Interaction{Options: []prompt.Option{}}
	view.Text, view.Truncated = interactionText(text)
	view.AwaitingReply = s.deps.HasPendingAsk != nil && s.deps.HasPendingAsk(id)
	if view.AwaitingReply && !view.Truncated {
		opts := prompt.WaitingOptions(view.Text)
		if prompt.OptionsReplyable(opts) && len(opts) <= 9 {
			view.Options = opts
		}
	}
	writeJSON(w, http.StatusOK, view)
}
