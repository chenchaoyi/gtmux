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
type Window struct {
	Label   string `json:"label"`    // "session" | "week (all models)" | "week (Fable)"
	PctUsed int    `json:"pct_used"` // 0–100, server-authoritative
	ResetAt string `json:"reset_at"` // human reset time, as reported ("Jul 17 at 10:59pm")
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
		return cached, hasCache // keep the last good snapshot on failure
	}
	r := Report{Windows: wins, At: now.Unix(), Warn: warnOf(wins, cfg.WarnPct)}
	save(r)
	return r, true
}

// warnOf returns the first weekly window at/over the warn threshold ("" = fine).
// Session (5h) windows are excluded — they reset hourly, so a high session % is
// normal and not a plan-exhaustion signal.
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
