// Package notify enqueues a desktop-notification request for the menu-bar app to
// deliver. gtmux no longer shells out to terminal-notifier or osascript: the
// native Gtmux.app drains this queue and posts a real UNUserNotificationCenter
// banner — sent as "Gtmux", clickable, with a Jump action and the agent icon.
//
// The hook process is short-lived, so this just drops a small JSON file and
// returns; the (long-running) app does the posting. Writing is atomic
// (temp file + rename) so the app never reads a half-written request.
package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Options describes one notification request.
type Options struct {
	Kind     string // "done" (finished) or "input" (needs you) — picks copy/sound
	Title    string // bold line, e.g. the session name
	Subtitle string // dimmer line, e.g. the agent name
	Message  string // body
	Pane     string // tmux pane id (%N) to jump to on click; "" → focus --last
	Session  string // session name (for the app's coalescing/threading)
	IconPath string // thumbnail image path; "" to omit
}

// request is the on-disk schema the menu-bar app decodes. Keep field names in
// sync with macapp NotificationManager.NotifyRequest.
type request struct {
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body"`
	Pane     string `json:"pane,omitempty"`
	Session  string `json:"session,omitempty"`
	Icon     string `json:"icon,omitempty"`
	TS       int64  `json:"ts"` // unix seconds; the app drops stale requests
}

// Send enqueues o for the menu-bar app to deliver. Best-effort and non-blocking:
// any error (no HOME, unwritable dir) is swallowed — a hook must never fail a turn.
func Send(o Options) {
	dir := state.NotifyDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(request{
		Kind:     o.Kind,
		Title:    o.Title,
		Subtitle: o.Subtitle,
		Body:     o.Message,
		Pane:     o.Pane,
		Session:  o.Session,
		Icon:     o.IconPath,
		TS:       time.Now().Unix(),
	})
	if err != nil {
		return
	}
	name := fmt.Sprintf("%d-%s.json", time.Now().UnixNano(), sanitize(o.Pane))
	final := filepath.Join(dir, name)
	tmp := filepath.Join(dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, final); err != nil { // atomic publish into the watched dir
		_ = os.Remove(tmp)
	}
}

// sanitize makes a pane id (e.g. "%12") safe for a filename.
func sanitize(s string) string {
	if s == "" {
		return "x"
	}
	return strings.NewReplacer("%", "p", "/", "_", ".", "_").Replace(s)
}
