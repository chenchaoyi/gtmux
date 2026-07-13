package hook

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// replyTailMax caps the reply summary recorded on a Stop event (a short "what did
// it just say" for HQ's triage — tens of tokens, not a transcript).
const replyTailMax = 200

// eventSummary computes the additive (summary, class) for an event record:
//   - UserPromptSubmit → the prompt's normalized head, so dispatch verify can match
//     the submission deterministically from the stream (no class).
//   - Stop → the reply tail + a deterministic asking/report class (incident ⑥).
//   - anything else → "", "".
//
// Best-effort: any missing source degrades to "" (a non-cooperative agent, an
// absent transcript). Deterministic — no LLM tokens.
func eventSummary(event, prompt, pane, agentSession, agentKey string) (summary, class string) {
	switch event {
	case "UserPromptSubmit":
		return dispatch.NormalizeHead(prompt), ""
	case "Stop":
		reply := lastReply(pane, agentSession, agentKey)
		if reply == "" {
			return "", ""
		}
		return replyTail(reply), classifyReply(reply)
	default:
		return "", ""
	}
}

// lastReply returns the assistant's most-recent reply text for a session, "" when
// none is resolvable. Prefers the hook's own session id; falls back to the pane's
// resume record (the digest's join).
func lastReply(pane, agentSession, agentKey string) string {
	key, sid := agentKey, agentSession
	if sid == "" && pane != "" {
		if loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}"); loc != "" {
			if rec, ok := resume.Load(loc); ok {
				key, sid = rec.Agent, rec.SessionID
			}
		}
	}
	if sid == "" {
		return ""
	}
	turns, err := transcript.Load(key, sid, 1)
	if err != nil || len(turns) == 0 {
		return ""
	}
	return turns[len(turns)-1].Response
}

// replyTail collapses whitespace and keeps the last replyTailMax runes (the tail is
// the most current part of a reply).
func replyTail(reply string) string {
	resp := strings.Join(strings.Fields(reply), " ")
	if r := []rune(resp); len(r) > replyTailMax {
		return "…" + strings.TrimSpace(string(r[len(r)-replyTailMax:]))
	}
	return resp
}

// questionScanLines is how many TRAILING prose lines classifyReply inspects for a
// question. A question posed to the user is frequently followed by a short status /
// sign-off line (or a usage footer), so checking only the final line misses it — the
// real dogfood bug. Six lines covers that tail without scanning the whole reply.
const questionScanLines = 6

// classifyReply is the deterministic turn-end classifier: "asking" when a question
// directed at the user appears in the reply's TRAILING BLOCK (any of the last
// questionScanLines prose lines ends with ?/？), else "report". Code fences, block
// quotes, and headings are skipped so a "?" inside a code sample or a rhetorical
// heading doesn't false-positive.
func classifyReply(reply string) string {
	lines := strings.Split(reply, "\n")
	inFence := false
	var prose []string
	for _, raw := range lines {
		ln := strings.TrimSpace(raw)
		if strings.HasPrefix(ln, "```") || strings.HasPrefix(ln, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || ln == "" || strings.HasPrefix(ln, ">") || strings.HasPrefix(ln, "#") {
			continue
		}
		prose = append(prose, ln)
	}
	start := len(prose) - questionScanLines
	if start < 0 {
		start = 0
	}
	for _, ln := range prose[start:] {
		ln = strings.TrimRight(ln, "`*_ )")
		if strings.HasSuffix(ln, "?") || strings.HasSuffix(ln, "？") {
			return "asking"
		}
	}
	return "report"
}
