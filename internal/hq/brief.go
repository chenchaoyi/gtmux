package hq

// The brief spool: colour-rendered supervisor reports, held until the target pane is not
// mid-turn, then written to its tty.
//
// WHY A SPOOL AT ALL. A TUI repaints the region its cursor sits in, so bytes written to
// the pane's tty while the agent is rendering are gone by its next frame. Measured before
// this was built: the identical write survived on an idle pane and vanished on a
// rendering one. HQ, however, issues its brief from INSIDE its own turn — precisely the
// worst moment — so writing on the spot is writing into a wipe.
//
// WHY NOT WAIT IN THE COMMAND. Because HQ's turn cannot end until its tool call returns.
// Blocking until quiet would block the condition being waited for: a deadlock, not a
// delay.
//
// HOW "QUIET" IS DECIDED — and this is the part worth keeping. NOT by watching the
// screen. The hook creates `active/<pane>` on UserPromptSubmit and removes it on Stop, so
// its existence IS "mid-turn", maintained by the agent's own lifecycle events. That file
// predicted both outcomes of the experiment exactly. Screen-shape inference is what this
// project has repeatedly had to unlearn; a marker written by the agent's own event is the
// deterministic answer to the same question.

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func briefDir() string { return filepath.Join(state.Dir(), "briefs") }

// PaneMidTurn reports whether the pane's agent is CURRENTLY producing a turn, from the
// hook's active marker.
//
// Absence is read as "not mid-turn", which is also the answer for a pane that never had a
// hook at all — deliberately the fail-OPEN direction: an unhooked agent gets its brief
// written immediately (today's behavior, possibly wiped) rather than queued into a spool
// nothing will ever drain, because the drain waits for a Stop that will never come.
func PaneMidTurn(pane string) bool { return state.Exists(state.ActivePath(pane)) }

// QueueBrief spools a rendered brief for a pane that is mid-turn.
func QueueBrief(pane, payload string) error {
	if err := os.MkdirAll(briefDir(), 0o755); err != nil {
		return err
	}
	// pane-<unixnano>: the pane is the drain key, the timestamp keeps ordering and makes
	// two briefs in one turn two files rather than one overwriting the other.
	name := strings.ReplaceAll(pane, "%", "pct") + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	return os.WriteFile(filepath.Join(briefDir(), name), []byte(payload), 0o644)
}

// WriteBriefTo writes rendered bytes to a tty.
func WriteBriefTo(tty, payload string) error {
	f, err := os.OpenFile(tty, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, payload)
	return err
}

// DrainBriefs flushes every spooled brief whose pane has finished its turn. Wired to the
// serve fast tick, beside the wake channel's own drain — same cadence, same reason.
//
// A brief whose pane is GONE is dropped rather than kept: it describes a moment that has
// passed, and a spool that only grows is its own defect.
func DrainBriefs() {
	ents, err := os.ReadDir(briefDir())
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(briefDir(), e.Name())
		pane := briefPaneOf(e.Name())
		if pane == "" {
			_ = os.Remove(path) // unparseable name: nothing can ever deliver it
			continue
		}
		tty := strings.TrimSpace(tmux.Display(pane, "#{pane_tty}"))
		if tty == "" {
			_ = os.Remove(path) // the pane is gone; the brief describes a past moment
			continue
		}
		if PaneMidTurn(pane) {
			continue // still rendering — its next frame would wipe us
		}
		b, err := os.ReadFile(path)
		if err != nil {
			_ = os.Remove(path)
			continue
		}
		if err := WriteBriefTo(tty, string(b)); err != nil {
			continue // keep it; a later tick retries
		}
		_ = os.Remove(path)
	}
}

// briefPaneOf recovers the pane id from a spool filename ("pct4-123…" → "%4").
func briefPaneOf(name string) string {
	i := strings.LastIndex(name, "-")
	if i <= 0 || !strings.HasPrefix(name, "pct") {
		return ""
	}
	return "%" + name[len("pct"):i]
}
