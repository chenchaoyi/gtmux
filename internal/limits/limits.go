// Package limits is the limits-watch layer (see openspec limits-watch): REAL
// subscription-window usage (Claude's 5-hour session window + weekly windows),
// obtained by running the agent's own `/usage` command headlessly and parsing
// its output — authoritative server data, not local estimation, via the user's
// sanctioned command (not a reverse-engineered endpoint).
//
// It spawns a process, so results are CACHED to state/limits.json with a TTL
// (15 min, shortened to 5 when a window is near its cap); the command is never
// run once per `gtmux usage`.
package limits

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Window is one subscription window (session / weekly / …).
//
// The two sources report a reset differently and both are kept: Claude's
// `/usage` prints a human string and nothing else, Codex's log gives an epoch.
// `ResetAt` stays the field every existing renderer reads; `ResetUnix` is added
// where it is known, because only a comparable time can tell a live reading from
// one that outlived its window.
type Window struct {
	Label     string `json:"label"`                // "claude session" | "claude week (all models)" | "codex week"
	PctUsed   int    `json:"pct_used"`             // 0–100, server-authoritative
	ResetAt   string `json:"reset_at"`             // human reset time, as reported ("Jul 17 at 10:59pm")
	Agent     string `json:"agent,omitempty"`      // which agent's plan this window belongs to
	ResetUnix int64  `json:"reset_unix,omitempty"` // epoch reset, when the source gives one
}

// Report is the cached limits snapshot.
type Report struct {
	Windows []Window `json:"windows"`
	At      int64    `json:"at"`   // epoch seconds the command last ran ok
	Warn    string   `json:"warn"` // the first window over the warn threshold, "" = fine
}

// Config controls the command + cadence + thresholds (from usage.json).
type Config struct {
	Command string // default "claude -p /usage"; "" disables. env-prefixable.
	TTLMin  int    // base cache TTL (default 15)
	NearMin int    // TTL when a window is near its cap (default 5)
	NearPct int    // "near" fraction (default 70)
	WarnPct int    // warn threshold (default 85)
}

// DefaultConfig is used when usage.json carries no limits keys.
var DefaultConfig = Config{Command: "claude -p /usage", TTLMin: 15, NearMin: 5, NearPct: 70, WarnPct: 85}

// path of the cache file.
func cachePath() string { return filepath.Join(state.Dir(), "limits.json") }

// Load returns the cached report (ok=false when there is no cache yet).
func Load() (Report, bool) {
	b, err := os.ReadFile(cachePath())
	if err != nil {
		return Report{}, false
	}
	var r Report
	if json.Unmarshal(b, &r) != nil {
		return Report{}, false
	}
	return r, true
}

// ttl chooses the cadence: NearMin when any window is at/over NearPct, else TTLMin.
func ttl(r Report, cfg Config) time.Duration {
	m := cfg.TTLMin
	for _, w := range r.Windows {
		if w.PctUsed >= cfg.NearPct {
			m = cfg.NearMin
			break
		}
	}
	return time.Duration(m) * time.Minute
}

// Fresh reports whether the cache is within its (near-aware) TTL as of now.
func Fresh(r Report, cfg Config, now time.Time) bool {
	if r.At == 0 {
		return false
	}
	return now.Sub(time.Unix(r.At, 0)) < ttl(r, cfg)
}

// Get returns limits, refreshing via the command only when the cache is stale
// (or force). When the command is disabled ("") or fails, the last good cache is
// returned (ok reflects whether ANY data is available). now is injectable.
func Get(cfg Config, force bool, now time.Time) (Report, bool) {
	cached, hasCache := Load()
	if cfg.Command == "" {
		return cached, hasCache
	}
	if hasCache && !force && Fresh(cached, cfg, now) {
		return cached, true
	}
	wins, err := runAndParse(cfg.Command)
	if err != nil || len(wins) == 0 {
		// The command is Claude's only route, and it fails for ordinary reasons —
		// `claude` missing from a launchd PATH is the one observed on this machine,
		// where `gtmux serve` runs with PATH=/usr/bin:/bin:/usr/sbin:/sbin.
		//
		// So ADD Codex's windows to the last good snapshot rather than replacing it.
		// Replacing made a transient failure delete Claude's plan from every surface
		// until the command worked again — the same loss `TestGetKeepsLastGoodCacheOnFailure`
		// exists to prevent, reintroduced by a path that did not exist when it was written.
		//
		// Not saved, deliberately: writing a fresh `At` here would mark a snapshot
		// built from a FAILURE as fresh and stop the command being retried for the
		// whole TTL.
		cx, ok := codexWindows(now)
		if !ok {
			return cached, hasCache
		}
		merged := append(othersThan("codex", cached.Windows), cx...)
		return Report{Windows: merged, At: cached.At, Warn: warnOf(merged, cfg.WarnPct)}, true
	}
	// The command route parses Claude's own `/usage` phrasing, so its windows are
	// Claude's.
	for i := range wins {
		wins[i].Agent = "claude"
		wins[i].Label = qualify("claude", wins[i].Label)
	}
	if cx, ok := codexWindows(now); ok {
		wins = append(wins, cx...)
	}
	r := Report{Windows: wins, At: now.Unix(), Warn: warnOf(wins, cfg.WarnPct)}
	save(r)
	return r, true
}

// warnOf returns the first weekly window at/over the warn threshold ("" = fine).
// Session (5h) windows are excluded — they reset hourly, so a high session % is
// normal and not a plan-exhaustion signal.
//
// The label carries the agent for anything but Claude ("codex week"), so a
// warning names WHOSE plan is tight. That matters where it is acted on: the
// spawn preflight suggests a cheaper model, and a suggestion drawn from the
// other agent's plan is advice about the wrong thing.
// othersThan returns the windows that do NOT belong to one agent — used to graft
// a fresh reading onto a cached snapshot without duplicating that agent's rows.
func othersThan(agent string, wins []Window) []Window {
	out := make([]Window, 0, len(wins))
	for _, w := range wins {
		if w.Agent != agent {
			out = append(out, w)
		}
	}
	return out
}

// qualify prefixes a window label with whose plan it is.
//
// EVERY window carries it, the first agent's included. Leaving one bare reads as
// the general case with a special case beside it — "session 10%" next to "codex
// session 0%" invites exactly the guess that the first one is everyone's. A label
// is the only thing most renderers show, so the label is where this has to be
// true, not just the `Agent` field a shipped client does not know about.
func qualify(agent, label string) string {
	if agent == "" || strings.HasPrefix(label, agent+" ") {
		return label
	}
	return agent + " " + label
}

func warnOf(wins []Window, warnPct int) string {
	for _, w := range wins {
		if strings.Contains(w.Label, "week") && w.PctUsed >= warnPct {
			return w.Label + " " + itoa(w.PctUsed) + "%"
		}
	}
	return ""
}

func save(r Report) {
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return
	}
	b, _ := json.Marshal(r)
	_ = os.WriteFile(cachePath(), b, 0o644)
}

// runAndParse executes the command (via the login shell so an env-prefixed
// string like `HTTPS_PROXY=… claude -p /usage` works) and parses its stdout.
func runAndParse(command string) ([]Window, error) {
	cmd := exec.Command(loginShell(), "-lc", agentenv.Wrap(command))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parse(string(out)), nil
}

// loginShell is the shell the limits command runs through.
//
// NOT `/bin/sh`, which is what this used and is why it failed under launchd. On
// macOS /bin/sh is bash in POSIX mode, and a POSIX login shell reads /etc/profile
// and ~/.profile ONLY — never ~/.bash_profile or ~/.bashrc, which is where a
// user's PATH addition actually lives. Measured on this machine: `claude` sits in
// ~/.local/bin, added by .bashrc/.zprofile; `sh -lc 'command -v claude'` cannot
// find it and `bash -lc` can. So `gtmux serve` — a LaunchAgent whose own PATH is
// /usr/bin:/bin:/usr/sbin:/sbin — could never run the command at all, and the
// subscription windows on every remote surface were only ever as fresh as the last
// time someone ran the CLI by hand.
//
// $SHELL is set even under launchd (verified on the running daemon), so it is the
// right answer. /bin/bash is the fallback because it reads strictly more startup
// files than /bin/sh, and an unusable $SHELL should not cost the feature.
func loginShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		if fi, err := os.Stat(s); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return s
		}
	}
	return "/bin/bash"
}

// itoa avoids importing strconv for one int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// Summary reduces the windows to ONE per plan — the tightest — for the places
// that have a line rather than a list.
//
// Codex doubled the window count, and every summary that simply joined them
// overflowed: the phone's header row truncated mid-number ("Fable 11…") and the
// `gtmux usage` footer ran past 100 characters. A summary that has to be cut off
// is not a summary.
//
// The tightest is the right one to keep because the question a summary answers is
// "where do I stand", and the answer is whichever window runs out first. Note this
// is deliberately NOT `warnOf`'s rule, which ignores session windows: that one
// decides whether to INTERRUPT you, and a 5-hour window at 90% is normal working
// and resets on its own. Showing is not warning.
//
// Order follows the input, so the display is stable across polls rather than
// reordering whenever a percentage crosses another.
func Summary(wins []Window) []Window {
	best := map[string]int{} // agent → index into out
	var out []Window
	for _, w := range wins {
		key := w.Agent
		if key == "" {
			key = w.Label // an older serve sent no agent; group by its own label
		}
		if i, seen := best[key]; seen {
			if w.PctUsed > out[i].PctUsed {
				out[i] = w
			}
			continue
		}
		best[key] = len(out)
		out = append(out, w)
	}
	return out
}
