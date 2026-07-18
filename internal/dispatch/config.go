package dispatch

import "github.com/chenchaoyi/gtmux/internal/usercfg"

// Tuning holds the dispatch timeouts/windows (all in seconds). Overridable via
// ~/.config/gtmux/config.json; every field falls back to a sane default.
type Tuning struct {
	ReadyTimeout      int64 // wait-for-agent-ready before delivering
	DeliverTimeout    int64 // confirm-landed timeout
	HookGrace         int64 // wait for a submit event before the screen fallback
	ResendWindow      int64 // re-send interlock window
	ReapIdleThreshold int64 // idle-after-work before a dispatch is a reap candidate
	ReapSnoozeTTL     int64 // default snooze duration for a declined reap suggestion
}

// DefaultTuning is the built-in configuration.
func DefaultTuning() Tuning {
	return Tuning{
		ReadyTimeout:      20,
		DeliverTimeout:    15,
		HookGrace:         3,
		ResendWindow:      90,
		ReapIdleThreshold: 30 * 60,
		ReapSnoozeTTL:     24 * 60 * 60,
	}
}

// LoadTuning reads overrides from config.json, keeping defaults for absent keys.
func LoadTuning() Tuning {
	t := DefaultTuning()
	var c struct {
		SpawnReadyTimeout   *int64 `json:"spawnReadyTimeout"`
		SpawnDeliverTimeout *int64 `json:"spawnDeliverTimeout"`
		SpawnHookGrace      *int64 `json:"spawnHookGrace"`
		ResendWindow        *int64 `json:"resendWindow"`
		ReapIdleThreshold   *int64 `json:"reapIdleThreshold"`
		ReapSnoozeTTL       *int64 `json:"reapSnoozeTTL"`
	}
	if usercfg.Load(&c) != nil {
		return t
	}
	setIf(&t.ReadyTimeout, c.SpawnReadyTimeout)
	setIf(&t.DeliverTimeout, c.SpawnDeliverTimeout)
	setIf(&t.HookGrace, c.SpawnHookGrace)
	setIf(&t.ResendWindow, c.ResendWindow)
	setIf(&t.ReapIdleThreshold, c.ReapIdleThreshold)
	setIf(&t.ReapSnoozeTTL, c.ReapSnoozeTTL)
	return t
}

// setIf overwrites *dst when src is a positive override.
func setIf(dst *int64, src *int64) {
	if src != nil && *src > 0 {
		*dst = *src
	}
}
