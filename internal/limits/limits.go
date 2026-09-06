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
		// The command is Claude's only route, so its failure loses Claude's
		// windows — but not Codex's, which cost nothing and come from a
		// different place entirely.
		if cx, ok := codexWindows(now); ok {
			r := Report{Windows: cx, At: now.Unix(), Warn: warnOf(cx, cfg.WarnPct)}
			save(r)
			return r, true
		}
		return cached, hasCache // keep the last good snapshot on failure
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
	cmd := exec.Command("/bin/sh", "-lc", agentenv.Wrap(command))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parse(string(out)), nil
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
