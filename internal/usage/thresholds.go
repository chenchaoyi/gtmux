package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Layers are PER AGENT TYPE, from ~/.config/gtmux/usage.json:
//
//	{"claude": {"ctxWarn": 0.8, "sessionOutWarn": 20000000,
//	            "typeRatePerMinWarn": 30000, "window": 200000},
//	 "horizonMin": 30}
//
// Absent file/keys → the defaults below. horizonMin is global (how far ahead
// the projection looks).
type Layers struct {
	CtxWarn            float64 `json:"ctxWarn"`            // live-context fraction
	SessionOutWarn     int64   `json:"sessionOutWarn"`     // cumulative output tokens per session
	TypeRatePerMinWarn int64   `json:"typeRatePerMinWarn"` // summed rate across the type's sessions
	Window             int64   `json:"window"`             // context window override (0 = auto)
}

// Defaults: generous — warn on genuinely unusual burn, not normal work.
var defaultLayers = Layers{CtxWarn: 0.8, SessionOutWarn: 20_000_000, TypeRatePerMinWarn: 30_000}

const defaultHorizon = 30 * time.Minute

type config struct {
	layers  map[string]Layers
	horizon time.Duration
}

// loadConfig reads usage.json once per call (tiny file; hooks are short-lived).
func loadConfig() config {
	cfg := config{layers: map[string]Layers{}, horizon: defaultHorizon}
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "usage.json"))
	if err != nil {
		return cfg
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		return cfg
	}
	for k, v := range raw {
		if k == "horizonMin" {
			var m float64
			if json.Unmarshal(v, &m) == nil && m > 0 {
				cfg.horizon = time.Duration(m * float64(time.Minute))
			}
			continue
		}
		var l Layers
		if json.Unmarshal(v, &l) == nil {
			cfg.layers[strings.ToLower(k)] = l
		}
	}
	return cfg
}

// layersFor merges the agent type's configured layers over the defaults.
func layersFor(agent string) Layers {
	cfg := loadConfig()
	l, ok := cfg.layers[strings.ToLower(agent)]
	if !ok {
		return defaultLayers
	}
	if l.CtxWarn <= 0 {
		l.CtxWarn = defaultLayers.CtxWarn
	}
	if l.SessionOutWarn <= 0 {
		l.SessionOutWarn = defaultLayers.SessionOutWarn
	}
	if l.TypeRatePerMinWarn <= 0 {
		l.TypeRatePerMinWarn = defaultLayers.TypeRatePerMinWarn
	}
	return l
}

func configWindow(agent string) int64 { return layersFor(agent).Window }

func horizon() time.Duration { return loadConfig().horizon }

// Evaluate returns the first breached-or-projected SESSION layer as a compact
// warn string ("" = fine). Pure over its inputs (unit-testable): ctxFrac and
// outTok are the current values; ratePerMin drives the projection; win is the
// context window (for projecting ctx growth from output tokens).
func Evaluate(l Layers, horizon time.Duration, win int64, ctxFrac float64, outTok, ratePerMin int64) string {
	h := horizon.Minutes()
	// Layer 1: live context — now, then projected (context grows at least as
	// fast as output lands in it).
	if ctxFrac >= l.CtxWarn {
		return fmt.Sprintf("ctx %d%%", int(ctxFrac*100))
	}
	if ratePerMin > 0 && win > 0 {
		growth := float64(ratePerMin) / float64(win) // ctx-fraction per minute
		if growth > 0 {
			etaMin := (l.CtxWarn - ctxFrac) / growth
			if etaMin <= h {
				return fmt.Sprintf("ctx→%d%% in ~%dm", int(l.CtxWarn*100), int(etaMin+0.5))
			}
		}
	}
	// Layer 2: session cumulative burn — now, then projected.
	if outTok >= l.SessionOutWarn {
		return fmt.Sprintf("burn %s", compactTok(outTok))
	}
	if ratePerMin > 0 {
		etaMin := float64(l.SessionOutWarn-outTok) / float64(ratePerMin)
		if etaMin <= h {
			return fmt.Sprintf("burn→%s in ~%dm", compactTok(l.SessionOutWarn), int(etaMin+0.5))
		}
	}
	return ""
}

// EvaluateSession is the impure convenience: layers+horizon from config.
func EvaluateSession(s Session) string {
	l := layersFor(s.Agent)
	win := windowFor(s.Agent, "", s.CtxTok)
	if l.Window > 0 {
		win = l.Window
	}
	return Evaluate(l, horizon(), win, s.CtxFrac, s.OutTok, s.RatePerMin)
}

// TypeRateWarn reports whether an agent TYPE's summed rate breaches its layer
// (the fleet-level check the rollup uses; sessions can't see this alone).
func TypeRateWarn(agent string, summedRate int64) string {
	l := layersFor(agent)
	if summedRate >= l.TypeRatePerMinWarn {
		return fmt.Sprintf("type rate %s/m", compactTok(summedRate))
	}
	return ""
}

// compactTok renders 5300000 → "5.3M", 12000 → "12k".
func compactTok(n int64) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
