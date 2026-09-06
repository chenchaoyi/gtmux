package limits

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Codex needs no command. It records the server's own rate-limit response into
// the session rollout, on the SAME `token_count` line the usage parser already
// reads, so the windows are there for free — no process, no headless one-shot.
//
// Measured on this machine (2026-09-06) before choosing this route:
//
//   - Claude carries NO subscription data on disk at all. `cost-state` in the
//     transcript is session COST, `~/.claude/stats-cache.json` is all-time model
//     totals; neither knows a window or a reset. Asking the server through the
//     user's own `/usage` is the only route, which is why that is what we do.
//   - Codex's `/usage` is a different thing from Claude's: an activity heatmap
//     (lifetime, peak, streak, a 12-month calendar), not remaining quota. So the
//     command route has no Codex counterpart even in principle.
//   - Codex's log does carry it: `primary` 300min + `secondary` 10080min, each
//     with `used_percent` and an epoch `resets_at`, plus `plan_type`. Live on the
//     current CLI (0.153.0).
//
// Two things that route has to respect, and both come from the same measurement:
//
//   - The window's identity is `window_minutes`, NEVER the field name. 41 blocks
//     on this machine carry `primary` = 10080, i.e. the WEEKLY window in the
//     field usually holding the 5-hour one. Reading position as identity would
//     mislabel them silently.
//   - A log is only as fresh as its last turn, so a reading can outlive its own
//     window. A window whose reset has passed is dropped rather than reported:
//     the percentage is genuinely unknown by then, and "unknown" is the honest
//     answer. Only an epoch makes that checkable, which Claude's human reset
//     string ("Sep 6 at 2:49pm") could not support.
//
// Absence is normal, not a failure: a session billed outside a plan window
// reports nulls, and so do Codex's own internal helper sessions.

// codexScanFiles bounds the directory walk: the newest rollouts by mtime, which
// is where an account-wide reading will be. Limits are per ACCOUNT, so any
// recent session's reading is as good as any other's.
const codexScanFiles = 8

// codexTailBytes is how much of a rollout's end to read looking for the newest
// `rate_limits`. They ride `token_count` events, which are frequent.
const codexTailBytes = 1 << 20

type codexRateLine struct {
	Timestamp string `json:"timestamp"`
	Payload   struct {
		Type   string `json:"type"`
		Limits *struct {
			Primary   *codexWindow `json:"primary"`
			Secondary *codexWindow `json:"secondary"`
			PlanType  string       `json:"plan_type"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

type codexWindow struct {
	UsedPercent float64 `json:"used_percent"`
	WindowMin   int     `json:"window_minutes"`
	ResetsAt    int64   `json:"resets_at"`
}

// codexWindows returns the newest rate-limit reading Codex has written, as
// windows. ok=false when no recent session carries one.
func codexWindows(now time.Time) ([]Window, bool) {
	for _, path := range recentCodexRollouts(codexScanFiles) {
		if wins, ok := codexWindowsIn(path, now); ok {
			return wins, true
		}
	}
	return nil, false
}

// codexWindowsIn reads one rollout's tail for its LAST rate-limit reading.
func codexWindowsIn(path string, now time.Time) ([]Window, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	from := fi.Size() - codexTailBytes
	if from < 0 {
		from = 0
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if int64(len(b)) > from {
		b = b[from:]
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !strings.Contains(line, `"rate_limits"`) {
			continue
		}
		var l codexRateLine
		if json.Unmarshal([]byte(line), &l) != nil || l.Payload.Limits == nil {
			continue
		}
		wins := codexToWindows(l.Payload.Limits.Primary, l.Payload.Limits.Secondary, now)
		if len(wins) > 0 {
			return wins, true
		}
	}
	return nil, false
}

func codexToWindows(primary, secondary *codexWindow, now time.Time) []Window {
	var out []Window
	for _, w := range []*codexWindow{primary, secondary} {
		if w == nil || w.WindowMin == 0 {
			continue
		}
		// A reading that outlived its own window says nothing about the window
		// running now.
		if w.ResetsAt > 0 && now.Unix() >= w.ResetsAt {
			continue
		}
		out = append(out, Window{
			Agent:     "codex",
			Label:     qualify("codex", windowName(w.WindowMin)),
			PctUsed:   int(w.UsedPercent + 0.5),
			ResetAt:   time.Unix(w.ResetsAt, 0).Format("Jan 2 at 3:04pm"),
			ResetUnix: w.ResetsAt,
		})
	}
	return out
}

// windowName names a window by its DURATION, which is the only thing that
// identifies it — see the note above about `primary` holding a weekly window.
func windowName(minutes int) string {
	switch {
	case minutes <= 60:
		return "hour"
	case minutes < 24*60:
		return "session"
	case minutes < 7*24*60:
		return "day"
	case minutes < 30*24*60:
		return "week"
	}
	return "month"
}

// recentCodexRollouts lists the newest rollout files, newest first.
func recentCodexRollouts(n int) []string {
	home := transcript.CodexHome()
	if home == "" {
		return nil
	}
	var all []string
	for _, pat := range []string{
		filepath.Join(home, "sessions", "*", "*", "*", "rollout-*.jsonl"),
		filepath.Join(home, "archived_sessions", "rollout-*.jsonl"),
	} {
		m, _ := filepath.Glob(pat)
		all = append(all, m...)
	}
	type ent struct {
		path string
		mod  time.Time
	}
	ents := make([]ent, 0, len(all))
	for _, p := range all {
		if fi, err := os.Stat(p); err == nil {
			ents = append(ents, ent{p, fi.ModTime()})
		}
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].mod.After(ents[j].mod) })
	if len(ents) > n {
		ents = ents[:n]
	}
	out := make([]string, len(ents))
	for i, e := range ents {
		out[i] = e.path
	}
	return out
}
