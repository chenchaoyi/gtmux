package hook

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// An agent's INTERNAL helper call (Codex's ambient-suggestions generator, its
// auto-mode safety classifier, …) runs as a complete pane-less "session": it reads
// the agent's own hooks config and fires SessionStart / UserPromptSubmit / Stop
// with a session id that belongs to no user conversation — no pane, no process we
// can find, no tokens, no transcript. Recorded as a native session it becomes a
// ghost radar row ("two codex sessions running") that the commander cannot act on:
// there is nothing to kill, and its `working` state never resolves because the
// helper's Stop doesn't pair back to the same "session".
//
// Detection is POSITIVE evidence, never the empty pane alone: a real native
// (non-tmux) session is pane-less too and must keep being sensed. The one thing
// that separates a helper call is its PROMPT — a known machine-issued system
// prompt — which only the UserPromptSubmit carries. So the filter marks the
// session at that event and swallows the rest of its lifecycle via the marker.

// helperPromptHeads are the known helper system-prompt heads, compared against the
// whitespace-normalized prompt prefix. Written in normalized form — copy a new
// fingerprint verbatim from the event's recorded summary (same normalization).
// Keep each long enough that no human-typed prompt plausibly starts with it.
var helperPromptHeads = []string{
	// Codex's ambient-suggestions generator (~/.codex/ambient-suggestions).
	"# Overview Generate 0 to 3 hyperpersonal",
	// Codex's auto-mode safety classifier ("Allowed by auto mode classifier").
	"You are an expert at upholding safety",
}

// isHelperPrompt reports whether a submitted prompt is a known agent-internal
// helper system prompt.
func isHelperPrompt(prompt string) bool {
	n := normalizeSpace(prompt)
	for _, h := range helperPromptHeads {
		if strings.HasPrefix(n, h) {
			return true
		}
	}
	return false
}

// normalizeSpace collapses whitespace runs to single spaces and trims — the same
// normalization the event summary records (dispatch.NormalizeHead's), so a
// fingerprint can be copied from the stream verbatim.
func normalizeSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// helperMarkerTTL bounds how long a helper-session marker is kept: a helper call
// lives seconds, but its terminating event may never arrive (the observed Stops
// carry a different/absent session id), so markers expire instead of relying on a
// SessionEnd that may not come.
const helperMarkerTTL = 24 * time.Hour

// helperDir is where helper-session markers live (one empty file per session id,
// same FS-safe encoding as the native store).
func helperDir() string { return filepath.Join(state.Dir(), "helper-sessions") }

func helperMarkerFor(sessionID string) string {
	return filepath.Join(helperDir(), base64.RawURLEncoding.EncodeToString([]byte(sessionID)))
}

// markHelperSession remembers a session id as an agent-internal helper call, so
// every later event it fires is swallowed. Pruning rides along here (the only
// writer) so markers whose session never signals an end can't accumulate.
func markHelperSession(sessionID string) {
	if sessionID == "" {
		return
	}
	if os.MkdirAll(helperDir(), 0o755) != nil {
		return
	}
	pruneHelperMarkers(time.Now())
	if f, err := os.Create(helperMarkerFor(sessionID)); err == nil {
		f.Close()
	}
}

// isHelperSession reports whether a session id was marked as a helper call.
func isHelperSession(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	_, err := os.Stat(helperMarkerFor(sessionID))
	return err == nil
}

// unmarkHelperSession drops a helper marker (its SessionEnd arrived).
func unmarkHelperSession(sessionID string) { _ = os.Remove(helperMarkerFor(sessionID)) }

// pruneHelperMarkers removes markers older than helperMarkerTTL.
func pruneHelperMarkers(now time.Time) {
	entries, err := os.ReadDir(helperDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) > helperMarkerTTL {
			_ = os.Remove(filepath.Join(helperDir(), e.Name()))
		}
	}
}
