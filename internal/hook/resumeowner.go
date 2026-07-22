package hook

import (
	"os"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Deciding WHICH conversation owns a pane (resume-record-ownership).
//
// The resume record answers one question: if this pane came back empty, which
// conversation should be relaunched into it? It is keyed by the pane's tmux locator, and
// until now ANY hook firing with that `$TMUX_PANE` overwrote it — so "a different session
// id" was treated as "the pane changed hands".
//
// That is wrong, because a coding agent runs some of its own machinery as SEPARATE
// sessions in the SAME pane. Claude Code runs a slash command like `/usage` as its own
// session id, in the same pane, firing the same hooks — and it would take the pane's
// record from the conversation actually running there. Observed on the reporter's own
// machine: `gtmux:0.0` pointed at a 3.5 KB `/usage` stub while the real conversation's
// log was 72 MB. Two consequences, the second worse than the first:
//
//  1. The chat view showed "no conversation history" for a pane mid-conversation.
//  2. `gtmux restore` would have resumed the STUB after a reboot — relaunching a `/usage`
//     screen in place of the work session, and the real conversation would be
//     unreachable from the record that was supposed to protect it.
//
// The rule below is the narrowest one that separates them: a session may take a pane it
// doesn't already own only if it is a real CONVERSATION — something the transcript parser
// can turn into at least one turn. A command stub has no turns, so there is nothing to
// resume into and no reason to hand it the pane.

// ownsPane reports whether `sid` may be recorded as the conversation running in `loc`.
//
// Cheap by construction: the common cases (no incumbent, or the same session reporting
// again) answer without touching a log at all, and the parse that follows only happens
// when a genuinely different session claims the pane.
func ownsPane(loc, agentKey, sid string) bool {
	prev, ok := resume.Load(loc)
	if !ok || prev.SessionID == "" || prev.SessionID == sid {
		return true // nobody to displace, or the same conversation moving on
	}
	// A different session claims the pane. Adopt it only if it is a conversation.
	if isConversation(agentKey, sid) {
		return true
	}
	// It isn't — but if the incumbent's log is gone (deleted, expired, or the record
	// predates a wipe), a pointer to nothing is worse than a pointer to a stub, so let
	// the claim through rather than pinning the pane to a conversation that can never
	// be resumed.
	return !logExists(prev.Agent, prev.SessionID)
}

// isConversation reports whether a session's log parses into at least one turn — the
// same parser the chat view uses, so "is there a conversation here" has exactly one
// definition in the product. A missing/unparseable log is not a conversation.
func isConversation(agentKey, sid string) bool {
	turns, err := transcript.Load(agentKey, sid, 1)
	return err == nil && len(turns) > 0
}

// logExists reports whether a session's log file is still on disk. Deliberately a stat,
// not a parse: this only decides whether an incumbent is dead, and a dead record must not
// cost the price of reading a live one.
func logExists(agentKey, sid string) bool {
	if sid == "" {
		return false
	}
	p := transcript.LogPath(agentKey, sid)
	if p == "" {
		return false // an agent whose layout we can't resolve: treat as gone
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
